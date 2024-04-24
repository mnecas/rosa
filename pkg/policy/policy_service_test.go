/*
Copyright (c) 2024 Red Hat, Inc.

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

package policy

import (
	"fmt"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/aws/aws-sdk-go-v2/aws"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/servicequotas"
	"github.com/aws/aws-sdk-go-v2/service/servicequotas/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	sdk "github.com/openshift-online/ocm-sdk-go"
	"github.com/openshift-online/ocm-sdk-go/logging"

	mock "github.com/openshift/rosa/pkg/aws"
	"github.com/openshift/rosa/pkg/aws/tags"
	"github.com/openshift/rosa/pkg/ocm"
)

var (
	awsClient *mock.MockClient
	ocmClient *ocm.Client
	policySvc PolicyService

	quota *servicequotas.GetServiceQuotaOutput
	role  *iamtypes.Role

	roleName, policyArn1, policyArn2 string
	policyArns                       []string
)

func TestDescribeUpgrade(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Policy Service suite")
}

var _ = Describe("Policy Service", func() {
	Context("Attach Policy", Ordered, func() {
		BeforeAll(func() {
			roleName = "sample-role"
			policyArn1 = "sample-policy-arn-1"
			policyArn2 = "sample-policy-arn-2"
			policyArns = []string{policyArn1, policyArn2}
			role = &iamtypes.Role{
				Tags: []iamtypes.Tag{
					{
						Key:   aws.String(tags.RedHatManaged),
						Value: aws.String("true"),
					},
				},
			}
			quotaValue := 2.0
			quota = &servicequotas.GetServiceQuotaOutput{
				Quota: &types.ServiceQuota{
					Value: &quotaValue,
				},
			}

			mockCtrl := gomock.NewController(GinkgoT())
			awsClient = mock.NewMockClient(mockCtrl)

			logger, err := logging.NewGoLoggerBuilder().
				Debug(true).
				Build()
			Expect(err).To(BeNil())
			// Set up the connection with the fake config
			connection, err := sdk.NewConnectionBuilder().
				Logger(logger).
				Tokens("").
				URL("http://fake.api").
				Build()
			Expect(err).To(BeNil())
			ocmClient = ocm.NewClientWithConnection(connection)

			policySvc = NewPolicyService(ocmClient, awsClient)
		})
		It("Test ValidateAttachOptions", func() {
			awsClient.EXPECT().GetRoleByName(roleName).Return(*role, nil)
			awsClient.EXPECT().GetIAMServiceQuota(QuotaCode).Return(quota, nil)
			awsClient.EXPECT().GetAttachedPolicy(aws.String(roleName)).Return([]mock.PolicyDetail{}, nil)
			awsClient.EXPECT().IsPolicyExists(policyArn1).Return(nil, nil)
			awsClient.EXPECT().IsPolicyExists(policyArn2).Return(nil, nil)
			err := policySvc.ValidateAttachOptions(roleName, policyArns)
			Expect(err).ShouldNot(HaveOccurred())
		})
		It("Test AutoAttachArbitraryPolicy", func() {
			awsClient.EXPECT().AttachRolePolicy(roleName, policyArn1).Return(nil)
			awsClient.EXPECT().AttachRolePolicy(roleName, policyArn2).Return(nil)
			output, err := policySvc.AutoAttachArbitraryPolicy(roleName, policyArns,
				"sample-account-id", "sample-org-id")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(output).To(Equal(fmt.Sprintf("Attached policy '%s' to role '%s'\n"+
				"Attached policy '%s' to role '%s'\n",
				policyArn1, roleName, policyArn2, roleName)))
		})
		It("Test ManualAttachArbitraryPolicy", func() {
			output := policySvc.ManualAttachArbitraryPolicy(roleName, policyArns,
				"sample-account-id", "sample-org-id")
			Expect(output).To(Equal(fmt.Sprintf(
				"aws iam attach-role-policy --role-name %s --policy-arn %s\n"+
					"aws iam attach-role-policy --role-name %s --policy-arn %s\n",
				roleName, policyArn1, roleName, policyArn2)))
		})
	})
})
