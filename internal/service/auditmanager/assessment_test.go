package auditmanager_test

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/auditmanager/types"
	sdkacctest "github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hashicorp/terraform-provider-aws/internal/acctest"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/create"
	tfauditmanager "github.com/hashicorp/terraform-provider-aws/internal/service/auditmanager"
	"github.com/hashicorp/terraform-provider-aws/names"
)

func TestAccAuditManagerAssessment_basic(t *testing.T) {
	var assessment types.Assessment
	rName := sdkacctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "aws_auditmanager_assessment.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			acctest.PreCheck(t)
			acctest.PreCheckPartitionHasService(names.AuditManagerEndpointID, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.AuditManagerEndpointID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckAssessmentDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAssessmentConfig_basic(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAssessmentExists(resourceName, &assessment),
					resource.TestCheckResourceAttr(resourceName, "name", rName),
					// TODO: check remaining required attributes are set
					acctest.MatchResourceAttrRegionalARN(resourceName, "arn", "auditmanager", regexp.MustCompile(`assessment/+.`)),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"roles"},
			},
		},
	})
}

func TestAccAuditManagerAssessment_disappears(t *testing.T) {
	var assessment types.Assessment
	rName := sdkacctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "aws_auditmanager_assessment.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			acctest.PreCheck(t)
			acctest.PreCheckPartitionHasService(names.AuditManagerEndpointID, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.AuditManagerEndpointID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckAssessmentDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAssessmentConfig_basic(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAssessmentExists(resourceName, &assessment),
					acctest.CheckFrameworkResourceDisappears(acctest.Provider, tfauditmanager.ResourceAssessment, resourceName),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func testAccCheckAssessmentDestroy(s *terraform.State) error {
	ctx := context.Background()
	conn := acctest.Provider.Meta().(*conns.AWSClient).AuditManagerClient

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "aws_auditmanager_assessment" {
			continue
		}

		_, err := tfauditmanager.FindAssessmentByID(ctx, conn, rs.Primary.ID)
		if err != nil {
			var nfe *types.ResourceNotFoundException
			if errors.As(err, &nfe) {
				return nil
			}
			return err
		}

		return create.Error(names.AuditManager, create.ErrActionCheckingDestroyed, tfauditmanager.ResNameAssessment, rs.Primary.ID, errors.New("not destroyed"))
	}

	return nil
}

func testAccCheckAssessmentExists(name string, assessment *types.Assessment) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return create.Error(names.AuditManager, create.ErrActionCheckingExistence, tfauditmanager.ResNameAssessment, name, errors.New("not found"))
		}

		if rs.Primary.ID == "" {
			return create.Error(names.AuditManager, create.ErrActionCheckingExistence, tfauditmanager.ResNameAssessment, name, errors.New("not set"))
		}

		ctx := context.Background()
		conn := acctest.Provider.Meta().(*conns.AWSClient).AuditManagerClient
		resp, err := tfauditmanager.FindAssessmentByID(ctx, conn, rs.Primary.ID)
		if err != nil {
			return create.Error(names.AuditManager, create.ErrActionCheckingExistence, tfauditmanager.ResNameAssessment, rs.Primary.ID, err)
		}

		*assessment = *resp

		return nil
	}
}

func testAccAssessmentConfigBase(rName string) string {
	return fmt.Sprintf(`
data "aws_caller_identity" "current" {}

resource "aws_s3_bucket" "test" {
  bucket = %[1]q
}

resource "aws_s3_bucket_acl" "test" {
  bucket = aws_s3_bucket.test.id
  acl    = "private"
}

resource "aws_iam_role" "test" {
  name = %[1]q

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Sid    = ""
        Principal = {
          Service = "ec2.amazonaws.com"
        }
      },
    ]
  })
}

resource "aws_auditmanager_control" "test" {
  name = %[1]q

  control_mapping_sources {
    source_name          = %[1]q
    source_set_up_option = "Procedural_Controls_Mapping"
    source_type          = "MANUAL"
  }
}

resource "aws_auditmanager_framework" "test" {
  name = %[1]q

  control_sets {
    name = %[1]q
    controls {
      id = aws_auditmanager_control.test.id
    }
  }
}
`, rName)
}

func testAccAssessmentConfig_basic(rName string) string {
	return acctest.ConfigCompose(
		testAccAssessmentConfigBase(rName),
		fmt.Sprintf(`
resource "aws_auditmanager_assessment" "test" {
  name = %[1]q
  
  assessment_reports_destination {
    destination      = "s3://${aws_s3_bucket.test.id}"
    destination_type = "S3"
  }
  
  framework_id = aws_auditmanager_framework.test.id
  
  roles {
    role_arn  = aws_iam_role.test.arn
    role_type = "PROCESS_OWNER"
  }
  
  scope {
    aws_accounts {
      id = data.aws_caller_identity.current.account_id
    }
    aws_services {
      service_name = "S3"
    }
  }
}
`, rName))
}
