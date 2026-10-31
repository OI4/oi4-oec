package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Gateway returns the IPv4 gateway of a network (GET /networks/{id}, IPAM):
// the address of the host in a bridge network (oec-infra run listens there
// on Linux).
func (c *Client) Gateway(ctx context.Context, network string) (string, error) {
	n, err := c.Network(ctx, network)
	if err != nil {
		return "", err
	}
	for _, cfg := range n.IPAM.Config {
		if ip := net.ParseIP(cfg.Gateway); ip != nil && ip.To4() != nil {
			return cfg.Gateway, nil
		}
	}
	return "", fmt.Errorf("network %s: no IPv4 gateway", network)
}

// Image is the subset of GET /images/{name}/json oec-infra run reports.
type Image struct {
	ID           string   `json:"Id"`
	RepoTags     []string `json:"RepoTags"`
	Os           string   `json:"Os"`
	Architecture string   `json:"Architecture"`
	Variant      string   `json:"Variant"`
}

// Image returns GET /images/{name}/json; name is a reference or an id.
// ErrNotFound: the engine does not have the image (D8 never pulls).
func (c *Client) Image(ctx context.Context, name string) (*Image, error) {
	segs := strings.Split(name, "/")
	for i, s := range segs {
		segs[i] = url.PathEscape(s)
	}
	var out Image
	if err := c.getJSON(ctx, "/images/"+strings.Join(segs, "/")+"/json", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateSpec is a container of oec-infra run. Binds are
// "<host path>:<container path>[:ro]"; ExtraHosts "<name>:<address>" (the
// address may be host-gateway).
type CreateSpec struct {
	Name        string
	Image       string
	Hostname    string
	Labels      map[string]string
	Binds       []string
	ExtraHosts  []string
	NetworkMode string
}

// Owned reports whether the name and the labels mark a container of
// oec-infra run.
func Owned(name string, labels map[string]string) bool {
	return strings.HasPrefix(strings.TrimPrefix(name, "/"), OwnPrefix) && labels[LabelOwner] == OwnerRun
}

// Create creates (does not start) a container of oec-infra run
// (POST /containers/create) and returns its id. ErrNotOwn: the name has
// not the prefix OwnPrefix or the label LabelOwner=OwnerRun is missing.
func (c *Client) Create(ctx context.Context, s CreateSpec) (string, error) {
	if !Owned(s.Name, s.Labels) {
		return "", fmt.Errorf("create %s: %w", s.Name, ErrNotOwn)
	}
	body, err := json.Marshal(map[string]any{
		"Image":    s.Image,
		"Hostname": s.Hostname,
		"Labels":   s.Labels,
		"HostConfig": map[string]any{
			"Binds":       s.Binds,
			"ExtraHosts":  s.ExtraHosts,
			"NetworkMode": s.NetworkMode,
		},
	})
	if err != nil {
		return "", err
	}
	out, err := c.do(ctx, http.MethodPost, "/containers/create", url.Values{"name": {s.Name}}, body)
	if err != nil {
		return "", err
	}
	var r struct {
		ID string `json:"Id"`
	}
	if err := json.Unmarshal(out, &r); err != nil || r.ID == "" {
		return "", fmt.Errorf("POST /containers/create: no id (%v)", err)
	}
	return r.ID, nil
}

// own inspects the container and refuses it unless it is one of
// oec-infra run.
func (c *Client) own(ctx context.Context, id string) (*Container, error) {
	ctr, err := c.Inspect(ctx, id)
	if err != nil {
		return nil, err
	}
	if !Owned(ctr.Name, ctr.Config.Labels) {
		return nil, fmt.Errorf("%s: %w", id, ErrNotOwn)
	}
	return ctr, nil
}

// notModified: the engine answers 304 (already started or stopped).
func notModified(err error) bool {
	var se *StatusError
	return errors.As(err, &se) && se.Code == http.StatusNotModified
}

// Start starts a container of oec-infra run.
func (c *Client) Start(ctx context.Context, id string) error {
	ctr, err := c.own(ctx, id)
	if err != nil {
		return err
	}
	if _, err := c.do(ctx, http.MethodPost, "/containers/"+url.PathEscape(ctr.ID)+"/start", nil, nil); err != nil && !notModified(err) {
		return err
	}
	return nil
}

// Stop stops a running container of oec-infra run: SIGTERM, after grace
// SIGKILL.
func (c *Client) Stop(ctx context.Context, id string, grace time.Duration) error {
	ctr, err := c.own(ctx, id)
	if err != nil {
		return err
	}
	if !ctr.State.Running {
		return nil
	}
	q := url.Values{"t": {strconv.Itoa(int(grace.Round(time.Second) / time.Second))}}
	if _, err := c.do(ctx, http.MethodPost, "/containers/"+url.PathEscape(ctr.ID)+"/stop", q, nil); err != nil && !notModified(err) {
		return err
	}
	return nil
}

// Remove removes a container of oec-infra run with its anonymous volumes
// (DELETE /containers/{id}?force=1&v=1); named volumes stay (it creates
// none).
func (c *Client) Remove(ctx context.Context, id string) error {
	ctr, err := c.own(ctx, id)
	if err != nil {
		return err
	}
	_, err = c.do(ctx, http.MethodDelete, "/containers/"+url.PathEscape(ctr.ID), url.Values{"force": {"1"}, "v": {"1"}}, nil)
	return err
}

// Own lists the containers of the run (every run if run is empty) that
// carry the marks of oec-infra run, running or not.
func (c *Client) Own(ctx context.Context, run string) ([]Summary, error) {
	label := []string{LabelOwner + "=" + OwnerRun}
	if run != "" {
		label = append(label, LabelRun+"="+run)
	}
	f, err := json.Marshal(map[string][]string{"label": label})
	if err != nil {
		return nil, err
	}
	var list []Summary
	if err := c.getJSON(ctx, "/containers/json", url.Values{"all": {"1"}, "filters": {string(f)}}, &list); err != nil {
		return nil, err
	}
	var out []Summary
	for _, s := range list {
		if Owned(s.Name(), s.Labels) && (run == "" || s.Labels[LabelRun] == run) {
			out = append(out, s)
		}
	}
	return out, nil
}
