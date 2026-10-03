// Package resolve turns resource references (UUIDs, or the composite
// template id) into human-readable names, so the CLI can display
// relationships by name instead of by UUID.
//
// It is the single helper shared by the `get` (and later `list`) commands.
// Every lookup goes through the SDK v2: the typed `library` services for the
// resources they expose, and the SDK's own REST client only for the endpoints
// those services do not wrap yet (vm-templates). It is the same single API
// boundary as the rest of the CLI, not a second HTTP client.
//
// Cost model (the rule the `get` commands rely on to stay free of N+1):
//   - a single reference (a container, a pool, a master, the SR of a VDI, the
//     VM/VDI of a VBD, the host/SR/pool of a PBD) costs one GET, constant;
//   - a collection (resident VMs, the PBDs of an SR, the VBDs of a VDI, the
//     hosts of a pool) must be resolved with one batch call + an id→name map
//     (see the `Names`-style batch helpers, added with the list views),
//     never with a GET per element.
//
// The client caches every fetched name, so resolving the same id several
// times within one command (for example one pool referenced by several
// resources) issues a single request.
package resolve

import (
	"context"
	"fmt"

	"github.com/gofrs/uuid"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/services/library"
	"github.com/vatesfr/xenorchestra-go-sdk/v2/client"
)

// templatesEndpoint is the REST resource that holds VM templates. The SDK v2
// does not (yet) expose a typed service for it, so templates are resolved
// through the SDK's own REST client — the same single API boundary. The
// missing operation should be contributed upstream.
const templatesEndpoint = "vm-templates"

// Client resolves resource references into names against one profile.
type Client struct {
	xo   library.Library
	http *client.Client

	cache map[string]string
}

// New builds a resolver on top of an authenticated SDK v2 client. lib is the
// typed service facade (vm/host/pool/sr), http is the SDK REST client used for
// the endpoints lib does not wrap yet (vm-templates).
func New(lib library.Library, http *client.Client) *Client {
	return &Client{
		xo:    lib,
		http:  http,
		cache: make(map[string]string),
	}
}

// HostName resolves a host UUID to its name_label, or the raw id when it is
// not a host (for example when a pool id is passed) or when the lookup fails.
func (c *Client) HostName(ctx context.Context, id uuid.UUID) (string, error) {
	if c.xo == nil || id.IsNil() {
		return id.String(), nil
	}
	if name, ok := c.lookup("host", id); ok {
		return name, nil
	}
	host, err := c.xo.Host().Get(ctx, id)
	if err != nil || host == nil {
		return id.String(), err
	}
	c.store("host", id, host.NameLabel)
	return host.NameLabel, nil
}

// PoolName resolves a pool UUID to its name_label, or the raw id when it is
// not a pool or when the lookup fails.
func (c *Client) PoolName(ctx context.Context, id uuid.UUID) (string, error) {
	if c.xo == nil || id.IsNil() {
		return id.String(), nil
	}
	if name, ok := c.lookup("pool", id); ok {
		return name, nil
	}
	pool, err := c.xo.Pool().Get(ctx, id)
	if err != nil || pool == nil {
		return id.String(), err
	}
	c.store("pool", id, pool.NameLabel)
	return pool.NameLabel, nil
}

// SRName resolves an SR UUID to its name_label, or the raw id when the lookup
// fails.
func (c *Client) SRName(ctx context.Context, id uuid.UUID) (string, error) {
	if c.xo == nil || id.IsNil() {
		return id.String(), nil
	}
	if name, ok := c.lookup("sr", id); ok {
		return name, nil
	}
	sr, err := c.xo.SR().Get(ctx, id)
	if err != nil || sr == nil {
		return id.String(), err
	}
	c.store("sr", id, sr.NameLabel)
	return sr.NameLabel, nil
}

// VMName resolves a VM UUID to its name_label, or the raw id when the lookup
// fails.
func (c *Client) VMName(ctx context.Context, id uuid.UUID) (string, error) {
	if c.xo == nil || id.IsNil() {
		return id.String(), nil
	}
	if name, ok := c.lookup("vm", id); ok {
		return name, nil
	}
	vm, err := c.xo.VM().GetByID(ctx, id)
	if err != nil || vm == nil {
		return id.String(), err
	}
	c.store("vm", id, vm.NameLabel)
	return vm.NameLabel, nil
}

// ResolveContainer resolves the container of a VM or SR, which is either a
// host or a pool. It tries the host first, then the pool: two requests at
// most, a constant cost independent of the pool size. It returns the raw id
// when neither lookup succeeds.
func (c *Client) ResolveContainer(ctx context.Context, id uuid.UUID) (string, error) {
	if c.xo == nil || id.IsNil() {
		return id.String(), nil
	}
	if name, err := c.HostName(ctx, id); err == nil && name != id.String() {
		return name, nil
	}
	if name, err := c.PoolName(ctx, id); err == nil && name != id.String() {
		return name, nil
	}
	return id.String(), fmt.Errorf("container %q not found", id)
}

// Template resolves the (composite) VM template id "poolId-templateUuid" to
// its name_label, or the raw id when the lookup fails. Templates are fetched
// through the SDK REST client because the typed service is not in the SDK yet.
func (c *Client) Template(ctx context.Context, id string) (string, error) {
	if c.http == nil || id == "" {
		return id, nil
	}
	if name, ok := c.lookupStr("template", id); ok {
		return name, nil
	}
	var t struct {
		NameLabel string `json:"name_label"`
	}
	if err := client.TypedGet(ctx, c.http, templatesEndpoint+"/"+id, struct{}{}, &t); err != nil {
		return id, err
	}
	if t.NameLabel == "" {
		return id, nil
	}
	c.storeStr("template", id, t.NameLabel)
	return t.NameLabel, nil
}

// lookup reports a previously resolved name for a UUID.
func (c *Client) lookup(kind string, id uuid.UUID) (string, bool) {
	v, ok := c.cache[kind+"/"+id.String()]
	return v, ok
}

// lookupStr reports a previously resolved name for a string key.
func (c *Client) lookupStr(kind, key string) (string, bool) {
	v, ok := c.cache[kind+"/"+key]
	return v, ok
}

// store remembers a resolved name so the same reference is fetched once.
func (c *Client) store(kind string, id uuid.UUID, name string) {
	c.cache[kind+"/"+id.String()] = name
}

// storeStr remembers a resolved name for a string key.
func (c *Client) storeStr(kind, key, name string) {
	c.cache[kind+"/"+key] = name
}
