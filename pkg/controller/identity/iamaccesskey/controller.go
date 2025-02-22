/*
Copyright 2020 The Crossplane Authors.

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

package iamaccesskey

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsiam "github.com/aws/aws-sdk-go-v2/service/iam"
	awsiamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/pkg/errors"
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

	"github.com/crossplane/provider-aws/apis/identity/v1alpha1"
	awsclient "github.com/crossplane/provider-aws/pkg/clients"
	"github.com/crossplane/provider-aws/pkg/clients/iam"
)

const (
	errUnexpectedObject = "The managed resource is not an IAMAccessKey resource"
	errList             = "failed to list IAMAccessKeys"
	errCreate           = "failed to create the IAMAccessKey resource"
	errDelete           = "failed to delete the IAMAccessKey resource"
	errUpdate           = "failed to update the IAMAccessKey resource"
)

// SetupIAMAccessKey adds a controller that reconciles IAMAccessKeys.
func SetupIAMAccessKey(mgr ctrl.Manager, l logging.Logger, rl workqueue.RateLimiter, poll time.Duration) error {
	name := managed.ControllerName(v1alpha1.IAMAccessKeyGroupKind)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(controller.Options{
			RateLimiter: ratelimiter.NewController(rl),
		}).
		For(&v1alpha1.IAMAccessKey{}).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(v1alpha1.IAMAccessKeyGroupVersionKind),
			managed.WithExternalConnecter(&connector{kube: mgr.GetClient(), newClientFn: iam.NewAccessClient}),
			managed.WithReferenceResolver(managed.NewAPISimpleReferenceResolver(mgr.GetClient())),
			managed.WithPollInterval(poll),
			managed.WithLogger(l.WithValues("controller", name)),
			managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name)))))
}

type connector struct {
	kube        client.Client
	newClientFn func(config aws.Config) iam.AccessClient
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cfg, err := awsclient.GetConfig(ctx, c.kube, mg, awsclient.GlobalRegion)
	if err != nil {
		return nil, err
	}
	return &external{client: c.newClientFn(*cfg), kube: c.kube}, nil
}

type external struct {
	client iam.AccessClient
	kube   client.Client
}

func (e *external) Observe(ctx context.Context, mgd resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mgd.(*v1alpha1.IAMAccessKey)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errUnexpectedObject)
	}

	if meta.GetExternalName(cr) == "" {
		return managed.ExternalObservation{}, nil
	}

	keys, err := e.client.ListAccessKeys(ctx, &awsiam.ListAccessKeysInput{UserName: aws.String(cr.Spec.ForProvider.IAMUsername)})
	if err != nil || len(keys.AccessKeyMetadata) == 0 {
		return managed.ExternalObservation{}, awsclient.Wrap(resource.Ignore(iam.IsErrorNotFound, err), errList)
	}
	found := false
	var accessKey awsiamtypes.AccessKeyMetadata
	for _, key := range keys.AccessKeyMetadata {
		if aws.ToString(key.AccessKeyId) == meta.GetExternalName(cr) {
			found = true
			accessKey = key
		}
	}
	if !found {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}
	switch accessKey.Status {
	case awsiamtypes.StatusTypeActive:
		cr.SetConditions(xpv1.Available())
	case awsiamtypes.StatusTypeInactive:
		cr.SetConditions(xpv1.Unavailable())
	}
	current := cr.Spec.ForProvider.Status
	cr.Spec.ForProvider.Status = awsclient.LateInitializeString(cr.Spec.ForProvider.Status, aws.String(string(accessKey.Status)))
	return managed.ExternalObservation{
		ResourceExists:          true,
		ResourceUpToDate:        string(accessKey.Status) == cr.Spec.ForProvider.Status,
		ResourceLateInitialized: current != cr.Spec.ForProvider.Status,
	}, nil
}

func (e *external) Create(ctx context.Context, mgd resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mgd.(*v1alpha1.IAMAccessKey)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errUnexpectedObject)
	}

	response, err := e.client.CreateAccessKey(ctx, &awsiam.CreateAccessKeyInput{UserName: aws.String(cr.Spec.ForProvider.IAMUsername)})
	if err != nil {
		return managed.ExternalCreation{}, awsclient.Wrap(err, errCreate)
	}

	var conn managed.ConnectionDetails
	if response != nil && response.AccessKey != nil {
		conn = managed.ConnectionDetails{
			xpv1.ResourceCredentialsSecretUserKey:     []byte(aws.ToString(response.AccessKey.AccessKeyId)),
			xpv1.ResourceCredentialsSecretPasswordKey: []byte(aws.ToString(response.AccessKey.SecretAccessKey)),
		}
	}
	meta.SetExternalName(cr, aws.ToString(response.AccessKey.AccessKeyId))
	return managed.ExternalCreation{ExternalNameAssigned: true, ConnectionDetails: conn}, nil
}

func (e *external) Update(ctx context.Context, mgd resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mgd.(*v1alpha1.IAMAccessKey)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errUnexpectedObject)
	}

	_, err := e.client.UpdateAccessKey(ctx, &awsiam.UpdateAccessKeyInput{
		AccessKeyId: aws.String(meta.GetExternalName(cr)),
		Status:      awsiamtypes.StatusType(cr.Spec.ForProvider.Status),
		UserName:    aws.String(cr.Spec.ForProvider.IAMUsername),
	})

	return managed.ExternalUpdate{}, awsclient.Wrap(err, errUpdate)
}

func (e *external) Delete(ctx context.Context, mgd resource.Managed) error {
	cr, ok := mgd.(*v1alpha1.IAMAccessKey)
	if !ok {
		return errors.New(errUnexpectedObject)
	}

	cr.Status.SetConditions(xpv1.Deleting())

	_, err := e.client.DeleteAccessKey(ctx, &awsiam.DeleteAccessKeyInput{
		UserName:    aws.String(cr.Spec.ForProvider.IAMUsername),
		AccessKeyId: aws.String(meta.GetExternalName(cr)),
	})

	return awsclient.Wrap(resource.Ignore(iam.IsErrorNotFound, err), errDelete)
}
