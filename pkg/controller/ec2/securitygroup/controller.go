/*
Copyright 2019 The Crossplane Authors.

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

package securitygroup

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsec2 "github.com/aws/aws-sdk-go-v2/service/ec2"
	awsec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane/provider-aws/apis/ec2/v1beta1"
	awsclient "github.com/crossplane/provider-aws/pkg/clients"
	"github.com/crossplane/provider-aws/pkg/clients/ec2"
)

const (
	errUnexpectedObject = "The managed resource is not an SecurityGroup resource"

	errDescribe         = "failed to describe SecurityGroup"
	errMultipleItems    = "retrieved multiple SecurityGroups for the given securityGroupId"
	errCreate           = "failed to create the SecurityGroup resource"
	errAuthorizeIngress = "failed to authorize ingress rules"
	errAuthorizeEgress  = "failed to authorize egress rules"
	errDelete           = "failed to delete the SecurityGroup resource"
	errSpecUpdate       = "cannot update spec of the SecurityGroup custom resource"
	errRevokeEgress     = "cannot remove the default egress rule"
	errStatusUpdate     = "cannot update status of the SecurityGroup custom resource"
	errUpdate           = "failed to update the SecurityGroup resource"
	errCreateTags       = "failed to create tags for the Security Group resource"
	errDeleteTags       = "failed to delete tags for the Security Group resource"
)

// SetupSecurityGroup adds a controller that reconciles SecurityGroups.
func SetupSecurityGroup(mgr ctrl.Manager, l logging.Logger, rl workqueue.RateLimiter, poll time.Duration) error {
	name := managed.ControllerName(v1beta1.SecurityGroupGroupKind)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(controller.Options{
			RateLimiter: ratelimiter.NewController(rl),
		}).
		For(&v1beta1.SecurityGroup{}).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(v1beta1.SecurityGroupGroupVersionKind),
			managed.WithExternalConnecter(&connector{kube: mgr.GetClient(), newClientFn: ec2.NewSecurityGroupClient}),
			managed.WithReferenceResolver(managed.NewAPISimpleReferenceResolver(mgr.GetClient())),
			managed.WithInitializers(),
			managed.WithConnectionPublishers(),
			managed.WithPollInterval(poll),
			managed.WithLogger(l.WithValues("controller", name)),
			managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name)))))
}

type connector struct {
	kube        client.Client
	newClientFn func(config aws.Config) ec2.SecurityGroupClient
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*v1beta1.SecurityGroup)
	if !ok {
		return nil, errors.New(errUnexpectedObject)
	}
	cfg, err := awsclient.GetConfig(ctx, c.kube, mg, aws.ToString(cr.Spec.ForProvider.Region))
	if err != nil {
		return nil, err
	}
	return &external{sg: c.newClientFn(*cfg), kube: c.kube}, nil
}

type external struct {
	sg   ec2.SecurityGroupClient
	kube client.Client
}

func (e *external) Observe(ctx context.Context, mgd resource.Managed) (managed.ExternalObservation, error) { // nolint:gocyclo
	cr, ok := mgd.(*v1beta1.SecurityGroup)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errUnexpectedObject)
	}

	if meta.GetExternalName(cr) == "" {
		return managed.ExternalObservation{}, nil
	}

	response, err := e.sg.DescribeSecurityGroups(ctx, &awsec2.DescribeSecurityGroupsInput{
		GroupIds: []string{meta.GetExternalName(cr)},
	})
	if err != nil {
		return managed.ExternalObservation{}, awsclient.Wrap(resource.Ignore(ec2.IsSecurityGroupNotFoundErr, err), errDescribe)
	}

	// in a successful response, there should be one and only one object
	if len(response.SecurityGroups) != 1 {
		return managed.ExternalObservation{}, errors.New(errMultipleItems)
	}

	observed := response.SecurityGroups[0]

	current := cr.Spec.ForProvider.DeepCopy()
	ec2.LateInitializeSG(&cr.Spec.ForProvider, &observed)

	cr.Status.AtProvider = ec2.GenerateSGObservation(observed)

	upToDate, err := ec2.IsSGUpToDate(cr.Spec.ForProvider, observed)
	if err != nil {
		return managed.ExternalObservation{}, awsclient.Wrap(err, errDescribe)
	}

	// this is to make sure that the security group exists with the specified traffic rules.
	if upToDate {
		cr.SetConditions(xpv1.Available())
	}

	return managed.ExternalObservation{
		ResourceExists:          true,
		ResourceUpToDate:        upToDate,
		ResourceLateInitialized: !cmp.Equal(current, &cr.Spec.ForProvider),
	}, nil
}

func (e *external) Create(ctx context.Context, mgd resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mgd.(*v1beta1.SecurityGroup)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errUnexpectedObject)
	}

	cr.Status.SetConditions(xpv1.Creating())
	if err := e.kube.Status().Update(ctx, cr); err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errStatusUpdate)
	}

	// Creating the SecurityGroup itself
	result, err := e.sg.CreateSecurityGroup(ctx, &awsec2.CreateSecurityGroupInput{
		GroupName:   aws.String(cr.Spec.ForProvider.GroupName),
		VpcId:       cr.Spec.ForProvider.VPCID,
		Description: aws.String(cr.Spec.ForProvider.Description),
	})
	if err != nil {
		return managed.ExternalCreation{}, awsclient.Wrap(err, errCreate)
	}
	en := aws.ToString(result.GroupId)
	// NOTE(muvaf): We have this code block in managed reconciler but this resource
	// has an exception where it needs to make another API call right after the
	// Create and we cannot afford losing the identifier in case RevokeSecurityGroupEgressRequest
	// fails.
	err = retry.OnError(retry.DefaultRetry, resource.IsAPIError, func() error {
		nn := types.NamespacedName{Name: cr.GetName()}
		if err := e.kube.Get(ctx, nn, cr); err != nil {
			return err
		}
		meta.SetExternalName(cr, en)
		return e.kube.Update(ctx, cr)
	})
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errSpecUpdate)
	}
	// NOTE(muvaf): AWS creates an initial egress rule and there is no way to
	// disable it with the create call. So, we revoke it right after the creation.
	_, err = e.sg.RevokeSecurityGroupEgress(ctx, &awsec2.RevokeSecurityGroupEgressInput{
		GroupId: aws.String(meta.GetExternalName(cr)),
		IpPermissions: []awsec2types.IpPermission{
			{
				IpProtocol: aws.String("-1"),
				IpRanges:   []awsec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}},
			},
		},
	})
	return managed.ExternalCreation{}, awsclient.Wrap(err, errRevokeEgress)
}

func (e *external) Update(ctx context.Context, mgd resource.Managed) (managed.ExternalUpdate, error) { // nolint:gocyclo
	cr, ok := mgd.(*v1beta1.SecurityGroup)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errUnexpectedObject)
	}

	response, err := e.sg.DescribeSecurityGroups(ctx, &awsec2.DescribeSecurityGroupsInput{
		GroupIds: []string{meta.GetExternalName(cr)},
	})
	if err != nil {
		return managed.ExternalUpdate{}, awsclient.Wrap(resource.Ignore(ec2.IsSecurityGroupNotFoundErr, err), errDescribe)
	}

	patch, err := ec2.CreateSGPatch(response.SecurityGroups[0], cr.Spec.ForProvider)
	if err != nil {
		return managed.ExternalUpdate{}, errors.New(errUpdate)
	}

	add, remove := awsclient.DiffEC2Tags(v1beta1.GenerateEC2Tags(cr.Spec.ForProvider.Tags), response.SecurityGroups[0].Tags)
	if len(remove) > 0 {
		if _, err := e.sg.DeleteTags(ctx, &awsec2.DeleteTagsInput{
			Resources: []string{meta.GetExternalName(cr)},
			Tags:      remove,
		}); err != nil {
			return managed.ExternalUpdate{}, awsclient.Wrap(err, errDeleteTags)
		}
	}

	if len(add) > 0 {
		if _, err := e.sg.CreateTags(ctx, &awsec2.CreateTagsInput{
			Resources: []string{meta.GetExternalName(cr)},
			Tags:      add,
		}); err != nil {
			return managed.ExternalUpdate{}, awsclient.Wrap(err, errCreateTags)
		}
	}

	if patch.Ingress != nil {
		if _, err := e.sg.AuthorizeSecurityGroupIngress(ctx, &awsec2.AuthorizeSecurityGroupIngressInput{
			GroupId:       aws.String(meta.GetExternalName(cr)),
			IpPermissions: ec2.GenerateEC2Permissions(cr.Spec.ForProvider.Ingress),
		}); err != nil && !ec2.IsRuleAlreadyExistsErr(err) {
			return managed.ExternalUpdate{}, awsclient.Wrap(err, errAuthorizeIngress)
		}
	}

	if patch.Egress != nil {
		if _, err = e.sg.AuthorizeSecurityGroupEgress(ctx, &awsec2.AuthorizeSecurityGroupEgressInput{
			GroupId:       aws.String(meta.GetExternalName(cr)),
			IpPermissions: ec2.GenerateEC2Permissions(cr.Spec.ForProvider.Egress),
		}); err != nil && !ec2.IsRuleAlreadyExistsErr(err) {
			return managed.ExternalUpdate{}, awsclient.Wrap(err, errAuthorizeEgress)
		}
	}

	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mgd resource.Managed) error {
	cr, ok := mgd.(*v1beta1.SecurityGroup)
	if !ok {
		return errors.New(errUnexpectedObject)
	}

	cr.Status.SetConditions(xpv1.Deleting())

	_, err := e.sg.DeleteSecurityGroup(ctx, &awsec2.DeleteSecurityGroupInput{
		GroupId: aws.String(meta.GetExternalName(cr)),
	})

	return awsclient.Wrap(resource.Ignore(ec2.IsSecurityGroupNotFoundErr, err), errDelete)
}
