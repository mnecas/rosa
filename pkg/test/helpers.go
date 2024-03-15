package test

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"

	"go.uber.org/mock/gomock"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	sdk "github.com/openshift-online/ocm-sdk-go"
	v1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	"github.com/openshift-online/ocm-sdk-go/logging"
	. "github.com/openshift-online/ocm-sdk-go/testing"
	"github.com/spf13/cobra"

	"github.com/openshift/rosa/pkg/aws"
	"github.com/openshift/rosa/pkg/ocm"
	"github.com/openshift/rosa/pkg/rosa"
)

func RunWithOutputCapture(runWithRuntime func(*rosa.Runtime, *cobra.Command) error,
	runtime *rosa.Runtime, cmd *cobra.Command) (string, string, error) {
	var err error
	var stdout []byte
	var stderr []byte

	rout, wout, _ := os.Pipe()
	tmpout := os.Stdout
	rerr, werr, _ := os.Pipe()
	tmperr := os.Stderr
	defer func() {
		os.Stdout = tmpout
		os.Stderr = tmperr
	}()
	os.Stdout = wout
	os.Stderr = werr

	go func() {
		err = runWithRuntime(runtime, cmd)
		wout.Close()
		werr.Close()
	}()
	stdout, _ = io.ReadAll(rout)
	stderr, _ = io.ReadAll(rerr)

	return string(stdout), string(stderr), err
}

func RunWithOutputCaptureAndArgv(runWithRuntime func(*rosa.Runtime, *cobra.Command, []string) error,
	runtime *rosa.Runtime, cmd *cobra.Command, argv *[]string) (string, string, error) {
	var err error
	var stdout []byte
	var stderr []byte

	rout, wout, _ := os.Pipe()
	tmpout := os.Stdout
	rerr, werr, _ := os.Pipe()
	tmperr := os.Stderr
	defer func() {
		os.Stdout = tmpout
		os.Stderr = tmperr
	}()
	os.Stdout = wout
	os.Stderr = werr

	go func() {
		err = runWithRuntime(runtime, cmd, *argv)
		wout.Close()
		werr.Close()
	}()
	stdout, _ = io.ReadAll(rout)
	stderr, _ = io.ReadAll(rerr)

	return string(stdout), string(stderr), err
}

var (
	MockClusterID   = "24vf9iitg3p6tlml88iml6j6mu095mh8"
	MockClusterHREF = "/api/clusters_mgmt/v1/clusters/24vf9iitg3p6tlml88iml6j6mu095mh8"
	MockClusterName = "cluster"
)

func BuildBreakGlassCredential() *v1.BreakGlassCredential {
	const breakGlassCredentialId = "test-id"
	breakGlassCredential, err := v1.NewBreakGlassCredential().
		ID(breakGlassCredentialId).Username("username").Status(v1.BreakGlassCredentialStatusIssued).
		Build()
	Expect(err).To(BeNil())
	return breakGlassCredential
}

func BuildExternalAuth() *v1.ExternalAuth {
	const externalAuthName = "microsoft-entra-id"
	externalAuth, err := v1.NewExternalAuth().ID(externalAuthName).
		Issuer(v1.NewTokenIssuer().URL("https://test.com").Audiences("abc")).
		Claim(v1.NewExternalAuthClaim().Mappings(v1.NewTokenClaimMappings().
			UserName(v1.NewUsernameClaim().Claim("username")).
			Groups(v1.NewGroupsClaim().Claim("groups")))).
		Build()
	Expect(err).To(BeNil())
	return externalAuth
}

func MockAutoscaler(modifyFn func(a *v1.ClusterAutoscalerBuilder)) *v1.ClusterAutoscaler {
	build := &v1.ClusterAutoscalerBuilder{}
	if modifyFn != nil {
		modifyFn(build)
	}

	autoscaler, err := build.Build()
	Expect(err).NotTo(HaveOccurred())
	return autoscaler
}

func MockCluster(modifyFn func(c *v1.ClusterBuilder)) *v1.Cluster {
	mock := v1.NewCluster().
		ID(MockClusterID).
		HREF(MockClusterHREF).
		Name(MockClusterName)

	if modifyFn != nil {
		modifyFn(mock)
	}

	cluster, err := mock.Build()
	Expect(err).NotTo(HaveOccurred())
	return cluster
}

func FormatClusterList(clusters []*v1.Cluster) string {
	var clusterJson bytes.Buffer

	v1.MarshalClusterList(clusters, &clusterJson)

	return fmt.Sprintf(`
	{
		"kind": "ClusterList",
		"page": 1,
		"size": %d,
		"total": %d,
		"items": %s
	}`, len(clusters), len(clusters), clusterJson.String())
}

func FormatIngressList(ingresses []*v1.Ingress) string {
	var ingressJson bytes.Buffer

	v1.MarshalIngressList(ingresses, &ingressJson)

	return fmt.Sprintf(`
	{
		"kind": "IngressList",
		"page": 1,
		"size": %d,
		"total": %d,
		"items": %s
	}`, len(ingresses), len(ingresses), ingressJson.String())
}

func FormatVersionList(versions []*v1.Version) string {
	var versionJson bytes.Buffer

	v1.MarshalVersionList(versions, &versionJson)

	return fmt.Sprintf(`
	{
		"kind": "VersionList",
		"page": 1,
		"size": %d,
		"total": %d,
		"items": %s
	}`, len(versions), len(versions), versionJson.String())
}

func FormatIDPList(idps []*v1.IdentityProvider) string {
	var idpJson bytes.Buffer

	v1.MarshalIdentityProviderList(idps, &idpJson)

	return fmt.Sprintf(`
	{
		"kind": "IdentityProviderList",
		"page": 1,
		"size": %d,
		"total": %d,
		"items": %s
	}`, len(idps), len(idps), idpJson.String())
}

func FormatHtpasswdUserList(htpasswdUsers []*v1.HTPasswdUser) string {
	var htpasswdUserJson bytes.Buffer

	v1.MarshalHTPasswdUserList(htpasswdUsers, &htpasswdUserJson)

	return fmt.Sprintf(`
	{
		"kind": "HTPasswdUserList",
		"page": 1,
		"size": %d,
		"total": %d,
		"items": %s
	}`, len(htpasswdUsers), len(htpasswdUsers), htpasswdUserJson.String())
}

func FormatBreakGlassCredentialList(credentials []*v1.BreakGlassCredential) string {
	var outputJson bytes.Buffer
	v1.MarshalBreakGlassCredentialList(credentials, &outputJson)
	return fmt.Sprintf(`
	{
		"kind": "BreakGlassCredentialsList",
		"page": 1,
		"size": %d,
		"total": %d,
		"items": %s
	}`, len(credentials), len(credentials), outputJson.String())
}

func FormatExternalAuthList(externalAuths []*v1.ExternalAuth) string {
	var outputJson bytes.Buffer

	v1.MarshalExternalAuthList(externalAuths, &outputJson)

	return fmt.Sprintf(`
	{
		"kind": "ExternalAuthList",
		"page": 1,
		"size": %d,
		"total": %d,
		"items": %s
	}`, len(externalAuths), len(externalAuths), outputJson.String())
}

func FormatNodePoolUpgradePolicyList(upgrades []*v1.NodePoolUpgradePolicy) string {
	var outputJson bytes.Buffer

	v1.MarshalNodePoolUpgradePolicyList(upgrades, &outputJson)

	return fmt.Sprintf(`
	{
		"kind": "NodePoolUpgradePolicyList",
		"page": 1,
		"size": %d,
		"total": %d,
		"items": %s
	}`, len(upgrades), len(upgrades), outputJson.String())
}

// FormatResource wraps the SDK marshalling and returns a string starting from an object
func FormatResource(resource interface{}) string {
	var outputJson bytes.Buffer
	var err error
	switch reflect.TypeOf(resource).String() {
	case "*v1.Version":
		if res, ok := resource.(*v1.Version); ok {
			err = v1.MarshalVersion(res, &outputJson)
		}
	case "*v1.NodePool":
		if res, ok := resource.(*v1.NodePool); ok {
			err = v1.MarshalNodePool(res, &outputJson)
		}
	case "*v1.MachinePool":
		if res, ok := resource.(*v1.MachinePool); ok {
			err = v1.MarshalMachinePool(res, &outputJson)
		}
	case "*v1.ClusterAutoscaler":
		if res, ok := resource.(*v1.ClusterAutoscaler); ok {
			err = v1.MarshalClusterAutoscaler(res, &outputJson)
		}
	case "*v1.ControlPlaneUpgradePolicy":
		if res, ok := resource.(*v1.ControlPlaneUpgradePolicy); ok {
			err = v1.MarshalControlPlaneUpgradePolicy(res, &outputJson)
		}
	default:
		{
			return "NOTIMPLEMENTED"
		}
	}
	if err != nil {
		return err.Error()
	}

	return outputJson.String()
}

func NewTestRuntime() *TestingRuntime {
	t := &TestingRuntime{}
	t.InitRuntime()
	return t
}

// TestingRuntime is a wrapper for the structure used for testing
type TestingRuntime struct {
	SsoServer   *ghttp.Server
	ApiServer   *ghttp.Server
	RosaRuntime *rosa.Runtime
}

func (t *TestingRuntime) InitRuntime() {
	// Create the servers:
	t.SsoServer = MakeTCPServer()
	t.ApiServer = MakeTCPServer()
	t.ApiServer.SetAllowUnhandledRequests(true)
	t.ApiServer.SetUnhandledRequestStatusCode(http.StatusInternalServerError)

	// Create the token:
	claims := MakeClaims()
	claims["username"] = "foo"
	accessTokenObj := MakeTokenObject(claims)
	accessToken := accessTokenObj.Raw

	// Prepare the server:
	t.SsoServer.AppendHandlers(
		RespondWithAccessToken(accessToken),
	)
	// Prepare the logger:
	logger, err := logging.NewGoLoggerBuilder().
		Debug(true).
		Build()
	Expect(err).To(BeNil())
	// Set up the connection with the fake config
	connection, err := sdk.NewConnectionBuilder().
		Logger(logger).
		Tokens(accessToken).
		URL(t.ApiServer.URL()).
		Build()
	// Initialize client object
	Expect(err).To(BeNil())
	ocmClient := ocm.NewClientWithConnection(connection)
	ocm.SetClusterKey("cluster1")
	t.RosaRuntime = rosa.NewRuntime()
	t.RosaRuntime.OCMClient = ocmClient
	t.RosaRuntime.Creator = &aws.Creator{
		ARN:       "fake",
		AccountID: "123",
		IsSTS:     false,
	}

	ctrl := gomock.NewController(GinkgoT())
	aws := aws.NewMockClient(ctrl)
	t.RosaRuntime.AWSClient = aws

	DeferCleanup(t.RosaRuntime.Cleanup)
	DeferCleanup(t.SsoServer.Close)
	DeferCleanup(t.ApiServer.Close)
	DeferCleanup(t.Close)
}

func (t *TestingRuntime) Close() {
	ocm.SetClusterKey("")
}

func (t *TestingRuntime) SetCluster(clusterKey string, cluster *v1.Cluster) {
	ocm.SetClusterKey(clusterKey)
	t.RosaRuntime.Cluster = cluster
	t.RosaRuntime.ClusterKey = clusterKey
}
