package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestBucketCreateInputPreservesDetachedEnvironment(t *testing.T) {
	payload, err := json.Marshal(BucketCreateInput{
		Name:      "test",
		ProjectId: "00000000-0000-0000-0000-000000000000",
	})
	if err != nil {
		t.Fatalf("marshal bucket create input: %v", err)
	}

	var input map[string]any
	if err := json.Unmarshal(payload, &input); err != nil {
		t.Fatalf("unmarshal bucket create input: %v", err)
	}
	environment, exists := input["environmentId"]
	if !exists || environment != nil {
		t.Fatalf("expected explicit null environment, got %s", payload)
	}
}

func TestAccBucketResourceDefault(t *testing.T) {
	projectName := "tf-acc-bucket-" + acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccBucketResourceConfig(projectName, "terraform-provider-bucket-test", "ams", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("railway_bucket.test", "name", "terraform-provider-bucket-test"),
					resource.TestCheckResourceAttr("railway_bucket_instance.test", "region", "ams"),
					resource.TestCheckResourceAttr("railway_bucket_instance.other", "region", "ams"),
					testAccCheckBucketDeployments(true),
				),
			},
			{
				ResourceName:      "railway_bucket.test",
				ImportState:       true,
				ImportStateIdFunc: bucketImportIDFunc,
				ImportStateVerify: true,
			},
			{
				ResourceName:      "railway_bucket_instance.test",
				ImportState:       true,
				ImportStateIdFunc: bucketInstanceImportIDFunc,
				ImportStateVerify: true,
			},
			{
				Config: testAccBucketResourceConfig(projectName, "terraform-provider-bucket-renamed", "ams", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("railway_bucket.test", "name", "terraform-provider-bucket-renamed"),
					testAccCheckBucketDeployments(true),
				),
			},
			{
				Config: testAccBucketResourceConfig(projectName, "terraform-provider-bucket-renamed", "ams", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("railway_bucket.test", "name", "terraform-provider-bucket-renamed"),
					testAccCheckBucketDeployments(false),
				),
			},
			{
				Config:      testAccBucketResourceConfig(projectName, "terraform-provider-bucket-renamed", "iad", false),
				ExpectError: regexp.MustCompile("Bucket Instance Region Cannot Be Changed"),
			},
		},
	})
}

func testAccBucketResourceConfig(
	projectName string,
	name string,
	region string,
	includeOtherInstance bool,
) string {
	otherInstance := ""
	if includeOtherInstance {
		otherInstance = fmt.Sprintf(`
resource "railway_bucket_instance" "other" {
  bucket_id      = railway_bucket.test.id
  environment_id = railway_environment.other.id
  region         = "%s"
}
`, region)
	}

	return fmt.Sprintf(`
resource "railway_project" "test" {
  name = "%s"

  default_environment = {
    name = "test"
  }
}

resource "railway_environment" "other" {
  name       = "other"
  project_id = railway_project.test.id
}

resource "railway_bucket" "test" {
  name       = "%s"
  project_id = railway_project.test.id
}

resource "railway_bucket_instance" "test" {
  bucket_id      = railway_bucket.test.id
  environment_id = railway_project.test.default_environment.id
  region         = "%s"
}
%s
`, projectName, name, region, otherInstance)
}

func testAccCheckBucketDeployments(expectOther bool) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		bucket := state.RootModule().Resources["railway_bucket.test"]
		defaultEnvironment := state.RootModule().Resources["railway_bucket_instance.test"]
		otherEnvironment := state.RootModule().Resources["railway_environment.other"]

		_, deployed, err := getManagedBucketDeployment(
			context.Background(),
			testAccClient(),
			defaultEnvironment.Primary.Attributes["environment_id"],
			bucket.Primary.ID,
		)
		if err != nil {
			return fmt.Errorf("read selected bucket deployment: %w", err)
		}
		if !deployed {
			return fmt.Errorf("bucket was not deployed to the selected environment")
		}

		_, deployed, err = getManagedBucketDeployment(
			context.Background(),
			testAccClient(),
			otherEnvironment.Primary.ID,
			bucket.Primary.ID,
		)
		if err != nil {
			return fmt.Errorf("read other bucket deployment: %w", err)
		}
		if deployed != expectOther {
			return fmt.Errorf("other bucket deployment: got %t, want %t", deployed, expectOther)
		}
		return nil
	}
}

func bucketImportIDFunc(state *terraform.State) (string, error) {
	rawState, ok := state.RootModule().Resources["railway_bucket.test"]
	if !ok {
		return "", fmt.Errorf("resource not found")
	}

	return fmt.Sprintf(
		"%s:%s",
		rawState.Primary.Attributes["project_id"],
		rawState.Primary.Attributes["id"],
	), nil
}

func bucketInstanceImportIDFunc(state *terraform.State) (string, error) {
	rawState, ok := state.RootModule().Resources["railway_bucket_instance.test"]
	if !ok {
		return "", fmt.Errorf("resource not found")
	}

	return fmt.Sprintf(
		"%s:%s",
		rawState.Primary.Attributes["environment_id"],
		rawState.Primary.Attributes["bucket_id"],
	), nil
}

func bucketCorsConfigurationImportIDFunc(state *terraform.State) (string, error) {
	rawState, ok := state.RootModule().Resources["railway_bucket_cors_configuration.test"]
	if !ok {
		return "", fmt.Errorf("resource not found")
	}

	return fmt.Sprintf(
		"%s:%s:%s",
		rawState.Primary.Attributes["project_id"],
		rawState.Primary.Attributes["environment_id"],
		rawState.Primary.Attributes["bucket_id"],
	), nil
}
