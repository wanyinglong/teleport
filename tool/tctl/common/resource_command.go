/*
Copyright 2015-2017 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	kyaml "k8s.io/client-go/1.4/pkg/util/yaml"
)

// GetCommand implements `tctl get` command
type GetCommand struct {
	config      *service.Config
	ref         services.Ref
	format      string
	namespace   string
	withSecrets bool
}

// GetCommand implements `tctl create` command
type CreateCommand struct {
	config   *service.Config
	filename string
}

// GetCommand implements `tctl delete` command
type DeleteCommand struct {
	config *service.Config
	ref    services.Ref
}

// Get prints one or many resources of a certain type
func (g *GetCommand) Get(client *auth.TunClient) error {
	collection, err := g.getCollection(client)
	if err != nil {
		return trace.Wrap(err)
	}
	switch g.format {
	case formatYAML:
		return collection.writeYAML(os.Stdout)

		// NOTE: only YAML is officially supported. Text and JSON are for experimentation only!
	case formatText:
		return collection.writeText(os.Stdout)
	case formatJSON:
		return collection.writeJSON(os.Stdout)
	}
	return trace.BadParameter("unsupported format")
}

// Create updates or insterts one or many resources
func (u *CreateCommand) Create(client *auth.TunClient) error {
	var reader io.ReadCloser
	var err error
	if u.filename != "" {
		reader, err = utils.OpenFile(u.filename)
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		reader = ioutil.NopCloser(os.Stdin)
	}
	decoder := kyaml.NewYAMLOrJSONDecoder(reader, 32*1024)
	count := 0
	for {
		var raw services.UnknownResource
		err := decoder.Decode(&raw)
		if err != nil {
			if err == io.EOF {
				if count == 0 {
					return trace.BadParameter("no resources found, emtpy input?")
				}
				return nil
			}
			return trace.Wrap(err)
		}
		count += 1
		switch raw.Kind {
		case services.KindSAMLConnector:
			conn, err := services.GetSAMLConnectorMarshaler().UnmarshalSAMLConnector(raw.Raw)
			if err != nil {
				return trace.Wrap(err)
			}
			if err := conn.CheckAndSetDefaults(); err != nil {
				return trace.Wrap(err)
			}
			if err := client.UpsertSAMLConnector(conn); err != nil {
				return trace.Wrap(err)
			}
			fmt.Printf("created SAML connector: %v\n", conn.GetName())
		case services.KindOIDCConnector:
			conn, err := services.GetOIDCConnectorMarshaler().UnmarshalOIDCConnector(raw.Raw)
			if err != nil {
				return trace.Wrap(err)
			}
			if err := client.UpsertOIDCConnector(conn); err != nil {
				return trace.Wrap(err)
			}
			fmt.Printf("created OIDC connector: %v\n", conn.GetName())
		case services.KindReverseTunnel:
			tun, err := services.GetReverseTunnelMarshaler().UnmarshalReverseTunnel(raw.Raw)
			if err != nil {
				return trace.Wrap(err)
			}
			if err := client.UpsertReverseTunnel(tun); err != nil {
				return trace.Wrap(err)
			}
			fmt.Printf("created reverse tunnel: %v\n", tun.GetName())
		case services.KindCertAuthority:
			ca, err := services.GetCertAuthorityMarshaler().UnmarshalCertAuthority(raw.Raw)
			if err != nil {
				return trace.Wrap(err)
			}
			if err := client.UpsertCertAuthority(ca); err != nil {
				return trace.Wrap(err)
			}
			fmt.Printf("created cert authority: %v \n", ca.GetName())
		case services.KindUser:
			user, err := services.GetUserMarshaler().UnmarshalUser(raw.Raw)
			if err != nil {
				return trace.Wrap(err)
			}
			if err := client.UpsertUser(user); err != nil {
				return trace.Wrap(err)
			}
			fmt.Printf("created user: %v\n", user.GetName())
		case services.KindRole:
			role, err := services.GetRoleMarshaler().UnmarshalRole(raw.Raw)
			if err != nil {
				return trace.Wrap(err)
			}
			err = role.CheckAndSetDefaults()
			if err != nil {
				return trace.Wrap(err)
			}
			if err := client.UpsertRole(role, backend.Forever); err != nil {
				return trace.Wrap(err)
			}
			fmt.Printf("created role: %v\n", role.GetName())
		case services.KindNamespace:
			ns, err := services.UnmarshalNamespace(raw.Raw)
			if err != nil {
				return trace.Wrap(err)
			}
			if err := client.UpsertNamespace(*ns); err != nil {
				return trace.Wrap(err)
			}
			fmt.Printf("created namespace: %v\n", ns.Metadata.Name)
		case services.KindTrustedCluster:
			tc, err := services.GetTrustedClusterMarshaler().Unmarshal(raw.Raw)
			if err != nil {
				return trace.Wrap(err)
			}
			if err := client.UpsertTrustedCluster(tc); err != nil {
				return trace.Wrap(err)
			}
			fmt.Printf("created trusted cluster: %q\n", tc.GetName())
		case "":
			return trace.BadParameter("missing resource kind")
		default:
			return trace.BadParameter("%q is not supported", raw.Kind)
		}
	}
}

// Delete deletes resource by name
func (d *DeleteCommand) Delete(client *auth.TunClient) error {
	if d.ref.Kind == "" {
		return trace.BadParameter("provide full resource name to delete e.g. roles/example")
	}
	if d.ref.Name == "" {
		return trace.BadParameter("provide full resource name to delete e.g. roles/example")
	}

	switch d.ref.Kind {
	case services.KindUser:
		if err := client.DeleteUser(d.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("user %v has been deleted\n", d.ref.Name)
	case services.KindSAMLConnector:
		if err := client.DeleteSAMLConnector(d.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("SAML Connector %v has been deleted\n", d.ref.Name)
	case services.KindOIDCConnector:
		if err := client.DeleteOIDCConnector(d.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("OIDC Connector %v has been deleted\n", d.ref.Name)
	case services.KindReverseTunnel:
		if err := client.DeleteReverseTunnel(d.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("reverse tunnel %v has been deleted\n", d.ref.Name)
	case services.KindRole:
		if err := client.DeleteRole(d.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("role %v has been deleted\n", d.ref.Name)
	case services.KindNamespace:
		if err := client.DeleteNamespace(d.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("namespace %v has been deleted\n", d.ref.Name)
	case services.KindTrustedCluster:
		if err := client.DeleteTrustedCluster(d.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("trusted cluster %q has been deleted\n", d.ref.Name)
	case "":
		return trace.BadParameter("missing resource kind")
	default:
		return trace.BadParameter("%q is not supported", d.ref.Kind)
	}

	return nil
}

func (g *GetCommand) getCollection(client auth.ClientI) (collection, error) {
	if g.ref.Kind == "" {
		return nil, trace.BadParameter("specify resource to list, e.g. 'tctl get roles'")
	}
	switch g.ref.Kind {
	case services.KindSAMLConnector:
		connectors, err := client.GetSAMLConnectors(g.withSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &samlCollection{connectors: connectors}, nil
	case services.KindOIDCConnector:
		connectors, err := client.GetOIDCConnectors(g.withSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &oidcCollection{connectors: connectors}, nil
	case services.KindReverseTunnel:
		tunnels, err := client.GetReverseTunnels()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &reverseTunnelCollection{tunnels: tunnels}, nil
	case services.KindCertAuthority:
		userAuthorities, err := client.GetCertAuthorities(services.UserCA, g.withSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		hostAuthorities, err := client.GetCertAuthorities(services.HostCA, g.withSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		userAuthorities = append(userAuthorities, hostAuthorities...)
		return &authorityCollection{cas: userAuthorities}, nil
	case services.KindUser:
		users, err := client.GetUsers()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &userCollection{users: users}, nil
	case services.KindNode:
		nodes, err := client.GetNodes(g.namespace)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &serverCollection{servers: nodes}, nil
	case services.KindAuthServer:
		servers, err := client.GetAuthServers()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &serverCollection{servers: servers}, nil
	case services.KindProxy:
		servers, err := client.GetAuthServers()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &serverCollection{servers: servers}, nil
	case services.KindRole:
		if g.ref.Name == "" {
			roles, err := client.GetRoles()
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &roleCollection{roles: roles}, nil
		}
		role, err := client.GetRole(g.ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &roleCollection{roles: []services.Role{role}}, nil
	case services.KindNamespace:
		if g.ref.Name == "" {
			namespaces, err := client.GetNamespaces()
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &namespaceCollection{namespaces: namespaces}, nil
		}
		ns, err := client.GetNamespace(g.ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &namespaceCollection{namespaces: []services.Namespace{*ns}}, nil
	case services.KindTrustedCluster:
		if g.ref.Name == "" {
			trustedClusters, err := client.GetTrustedClusters()
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &trustedClusterCollection{trustedClusters: trustedClusters}, nil
		}
		trustedCluster, err := client.GetTrustedCluster(g.ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &trustedClusterCollection{trustedClusters: []services.TrustedCluster{trustedCluster}}, nil
	}

	return nil, trace.BadParameter("'%v' is not supported", g.ref.Kind)
}

const (
	formatYAML = "yaml"
	formatText = "text"
	formatJSON = "json"
)
