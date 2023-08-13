//go:build (all || resource_environment_resource_kubernetes) && !exclude_resource_environment_resource_kubernetes
// +build all resource_environment_resource_kubernetes
// +build !exclude_resource_environment_resource_kubernetes

package acceptancetests

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/taskagent"
	"github.com/microsoft/terraform-provider-azuredevops/azuredevops/internal/acceptancetests/testutils"
	"github.com/microsoft/terraform-provider-azuredevops/azuredevops/internal/client"
)

// Verifies that the following sequence of events occurs without error:
//
//	(1) TF apply creates resource
//	(2) TF state values are set
//	(3) Resource can be queried by ID and has expected information
//	(4) TF apply updates resource with new name
//	(5) Resource can be queried by ID and has expected name
//	(6) TF destroy deletes resource
//	(7) Resource can no longer be queried by ID
func TestAccEnvironmentResourceKubernetes_CreateAndUpdate(t *testing.T) {
	projectName := testutils.GenerateResourceName()
	environmentName := testutils.GenerateResourceName()
	serviceEndpointName := testutils.GenerateResourceName()
	resourceNameFirst := testutils.GenerateResourceName()
	resourceNameSecond := testutils.GenerateResourceName()
	tfNode := "azuredevops_environment_resource_kubernetes.kubernetes"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testutils.PreCheck(t, nil) },
		Providers:    testutils.GetProviders(),
		CheckDestroy: checkEnvironmentResourceDestroyed,
		Steps: []resource.TestStep{
			{
				Config: testutils.HclEnvironmentResourceKubernetesResource(projectName, environmentName, serviceEndpointName, resourceNameFirst),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(tfNode, "name", resourceNameFirst),
					resource.TestCheckResourceAttrSet(tfNode, "project_id"),
					resource.TestCheckResourceAttrSet(tfNode, "environment_id"),
					resource.TestCheckResourceAttrSet(tfNode, "service_endpoint_id"),
					checkEnvironmentResourceExists(tfNode, resourceNameFirst),
				),
			},
			{
				Config: testutils.HclEnvironmentResourceKubernetesResource(projectName, environmentName, serviceEndpointName, resourceNameSecond),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(tfNode, "name", resourceNameSecond),
					resource.TestCheckResourceAttrSet(tfNode, "project_id"),
					resource.TestCheckResourceAttrSet(tfNode, "environment_id"),
					resource.TestCheckResourceAttrSet(tfNode, "service_endpoint_id"),
					checkEnvironmentResourceExists(tfNode, resourceNameSecond),
				),
			},
		},
	})
}

// Given the name of a resource, this will return a function that will check whether
// the resource (1) exists in the state and (2) exist in AzDO and (3) has the correct name
func checkEnvironmentResourceExists(tfNode string, expectedName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		resource, ok := s.RootModule().Resources[tfNode]
		if !ok {
			return fmt.Errorf("Did not find an resource in the TF state")
		}

		clients := testutils.GetProvider().Meta().(*client.AggregatedClient)
		id, err := strconv.Atoi(resource.Primary.ID)
		if err != nil {
			return fmt.Errorf("Parse resource id, ID:  %v !. Error= %v", resource.Primary.ID, err)
		}
		projectId := resource.Primary.Attributes["project_id"]
		environmentIdStr := resource.Primary.Attributes["environment_id"]
		environmentId, err := strconv.Atoi(environmentIdStr)
		if err != nil {
			return fmt.Errorf("Parse environment_id error, ID:  %v !. Error= %v", environmentIdStr, err)
		}

		readResource, err := readEnvironmentResourceKubernetes(clients, projectId, environmentId, id)
		if err != nil {
			return fmt.Errorf("Resource with ID=%d cannot be found!. Error=%v", id, err)
		}

		if *readResource.Name != expectedName {
			return fmt.Errorf("Resource with ID=%d has Name=%s, but expected Name=%s", id, *readResource.Name, expectedName)
		}

		return nil
	}
}

// verifies that environment referenced in the state is destroyed. This will be invoked
// *after* terraform destroys the resource but *before* the state is wiped clean.
func checkEnvironmentResourceDestroyed(s *terraform.State) error {
	clients := testutils.GetProvider().Meta().(*client.AggregatedClient)

	// verify that every environment referenced in the state does not exist in AzDO
	for _, resource := range s.RootModule().Resources {
		if resource.Type != "resource_environment_resource_kubernetes" {
			continue
		}

		id, err := strconv.Atoi(resource.Primary.ID)
		if err != nil {
			return fmt.Errorf("Parse resource id, ID:  %v !. Error= %v", resource.Primary.ID, err)
		}
		projectId := resource.Primary.Attributes["project_id"]
		environmentIdStr := resource.Primary.Attributes["environment_id"]
		environmentId, err := strconv.Atoi(environmentIdStr)
		if err != nil {
			return fmt.Errorf("Parse environment_id error, ID:  %v !. Error= %v", environmentIdStr, err)
		}

		// indicates the environment still exists - this should fail the test
		if _, err := readEnvironmentResourceKubernetes(clients, projectId, environmentId, id); err == nil {
			return fmt.Errorf("Resource ID %d should not exist", id)
		}
	}

	return nil
}

// Lookup an Environment using the ID and the project ID.
func readEnvironmentResourceKubernetes(clients *client.AggregatedClient, projectId string, environmentId int, resourceId int) (*taskagent.KubernetesResource, error) {
	return clients.TaskAgentClient.GetKubernetesResource(clients.Ctx,
		taskagent.GetKubernetesResourceArgs{
			Project:       &projectId,
			EnvironmentId: &environmentId,
			ResourceId:    &resourceId,
		},
	)
}
