/*
Copyright 2017 Gravitational, Inc.

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

package services

// ClusterConfiguration stores the cluster configuration in the backend. All
// the resources modified by this interface can only have a single instance
// in the backend.
type ClusterConfiguration interface {
	// SetClusterName sets the name of the cluster.
	SetClusterName(string) error
	// GetClusterName gets the name of the cluster.
	GetClusterName() (string, error)

	// SetStaticTokens sets the list of static tokens used to provision nodes.
	SetStaticTokens([]ProvisionToken) error
	// GetStaticTokens gets the list of static tokens used to provision nodes.
	GetStaticTokens() ([]ProvisionToken, error)

	// GetAuthPreference returns the authentication preferences for a cluster.
	GetAuthPreference() (AuthPreference, error)
	// SetAuthPreference sets the authentication preferences for a cluster.
	SetAuthPreference(AuthPreference) error
}
