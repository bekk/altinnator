package controller_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	altinnatorv1alpha1 "github.com/kartverket/altinnator/api/v1alpha1"
	"github.com/kartverket/altinnator/internal/controller"
	"github.com/kartverket/altinnator/pkg/altinn"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

type write struct {
	id      string
	content altinn.MaskinportenSchemaContent
}

type fakeRegistry struct {
	inAltinn    *altinn.MaskinportenSchema
	getErr      error
	createErr   error
	updateErr   error
	policyErr   error
	withdrawErr error
	created     []write
	updated     []write
	policies    []string
	withdrawn   []string
}

func (f *fakeRegistry) GetMaskinportenSchema(_ context.Context, id string) (*altinn.MaskinportenSchema, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.inAltinn == nil {
		return nil, fmt.Errorf("resource %s: %w", id, altinn.ErrNotFound)
	}
	return f.inAltinn, nil
}

func (f *fakeRegistry) CreateMaskinportenSchema(
	_ context.Context, id string, content altinn.MaskinportenSchemaContent,
) error {
	f.created = append(f.created, write{id, content})
	return f.createErr
}

func (f *fakeRegistry) UpdateMaskinportenSchema(
	_ context.Context, id string, content altinn.MaskinportenSchemaContent,
) error {
	f.updated = append(f.updated, write{id, content})
	return f.updateErr
}

func (f *fakeRegistry) SetMaskinportenSchemaPolicy(_ context.Context, id string) error {
	f.policies = append(f.policies, id)
	return f.policyErr
}

func (f *fakeRegistry) WithdrawMaskinportenSchema(_ context.Context, id string) error {
	f.withdrawn = append(f.withdrawn, id)
	return f.withdrawErr
}

var _ = Describe("MaskinportenSchemaResourceReconciler", func() {
	key := types.NamespacedName{Namespace: "matrikkel", Name: "brukernavn"}

	var (
		resource   *altinnatorv1alpha1.MaskinportenSchemaResource
		registry   *fakeRegistry
		kubeClient client.Client
		intercept  interceptor.Funcs
	)

	BeforeEach(func() {
		resource = &altinnatorv1alpha1.MaskinportenSchemaResource{
			ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name, Generation: 1},
			Spec: altinnatorv1alpha1.MaskinportenSchemaResourceSpec{
				ResourceID:            "kv-matrikkel-brukernavn",
				Scopes:                []string{"kartverk:matrikkel:brukernavn"},
				Title:                 altinnatorv1alpha1.MultilingualText{En: "Cadastre API", Nb: "Matrikkel API", Nn: "Matrikkel API"},
				Description:           altinnatorv1alpha1.MultilingualText{En: "The Cadastre", Nb: "Matrikkelen", Nn: "Matrikkelen"},
				DelegationDescription: altinnatorv1alpha1.MultilingualText{En: "Allows", Nb: "Gir", Nn: "Gir"},
				Homepage:              "https://kartverket.no/matrikkel",
				ContactPoints:         []altinnatorv1alpha1.ContactPoint{{Category: "Kundesenter", Email: "post@kartverket.no"}},
				Keywords:              []altinnatorv1alpha1.Keyword{{Word: "matrikkel", Language: "nb"}},
			},
		}
		registry = &fakeRegistry{}
		intercept = interceptor.Funcs{}
	})

	sameAsSpec := func() *altinn.MaskinportenSchema {
		return &altinn.MaskinportenSchema{
			ResourceID:            "kv-matrikkel-brukernavn",
			Scopes:                []string{"kartverk:matrikkel:brukernavn"},
			Title:                 altinn.MultilingualText{En: "Cadastre API", Nb: "Matrikkel API", Nn: "Matrikkel API"},
			Description:           altinn.MultilingualText{En: "The Cadastre", Nb: "Matrikkelen", Nn: "Matrikkelen"},
			DelegationDescription: altinn.MultilingualText{En: "Allows", Nb: "Gir", Nn: "Gir"},
			Homepage:              "https://kartverket.no/matrikkel",
			ContactPoints:         []altinn.ContactPoint{{Category: "Kundesenter", Email: "post@kartverket.no"}},
			Keywords:              []altinn.Keyword{{Word: "matrikkel", Language: "nb"}},
		}
	}

	markReady := func() {
		resource.Status.Conditions = []metav1.Condition{{
			Type: "Ready", Status: metav1.ConditionTrue, Reason: "Synced", ObservedGeneration: 1,
			LastTransitionTime: metav1.Now(),
		}}
	}

	reconcile := func() (ctrl.Result, error) {
		scheme := runtime.NewScheme()
		Expect(altinnatorv1alpha1.AddToScheme(scheme)).To(Succeed())
		kubeClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(resource).
			WithStatusSubresource(resource).
			WithInterceptorFuncs(intercept).
			Build()
		reconciler := &controller.MaskinportenSchemaResourceReconciler{Client: kubeClient, Registry: registry}
		return reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: key})
	}

	markDeleting := func() {
		resource.Finalizers = []string{"altinnator.kartverket.no/withdraw"}
		resource.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	}

	isGone := func() bool {
		err := kubeClient.Get(context.Background(), key, new(altinnatorv1alpha1.MaskinportenSchemaResource))
		return apierrors.IsNotFound(err)
	}

	readyCondition := func() *metav1.Condition {
		saved := new(altinnatorv1alpha1.MaskinportenSchemaResource)
		Expect(kubeClient.Get(context.Background(), key, saved)).To(Succeed())
		return meta.FindStatusCondition(saved.Status.Conditions, "Ready")
	}

	It("adds the finalizer before it changes anything in Altinn", func() {
		_, err := reconcile()

		Expect(err).NotTo(HaveOccurred())
		saved := new(altinnatorv1alpha1.MaskinportenSchemaResource)
		Expect(kubeClient.Get(context.Background(), key, saved)).To(Succeed())
		Expect(saved.Finalizers).To(Equal([]string{"altinnator.kartverket.no/withdraw"}))
	})

	It("keeps a single finalizer when it is already there", func() {
		resource.Finalizers = []string{"altinnator.kartverket.no/withdraw"}

		_, err := reconcile()

		Expect(err).NotTo(HaveOccurred())
		saved := new(altinnatorv1alpha1.MaskinportenSchemaResource)
		Expect(kubeClient.Get(context.Background(), key, saved)).To(Succeed())
		Expect(saved.Finalizers).To(Equal([]string{"altinnator.kartverket.no/withdraw"}))
	})

	It("does not touch Altinn when the finalizer cannot be added", func() {
		intercept.Update = func(
			context.Context, client.WithWatch, client.Object, ...client.UpdateOption,
		) error {
			return errors.New("conflict")
		}

		_, err := reconcile()

		Expect(err).To(HaveOccurred())
		Expect(registry.created).To(BeEmpty())
	})

	It("creates a new resource in Altinn and sets its policy", func() {
		_, err := reconcile()

		Expect(err).NotTo(HaveOccurred())
		Expect(registry.created).To(Equal([]write{{"kv-matrikkel-brukernavn", altinn.MaskinportenSchemaContent{
			Scopes:                []string{"kartverk:matrikkel:brukernavn"},
			Title:                 altinn.MultilingualText{En: "Cadastre API", Nb: "Matrikkel API", Nn: "Matrikkel API"},
			Description:           altinn.MultilingualText{En: "The Cadastre", Nb: "Matrikkelen", Nn: "Matrikkelen"},
			DelegationDescription: altinn.MultilingualText{En: "Allows", Nb: "Gir", Nn: "Gir"},
			Homepage:              "https://kartverket.no/matrikkel",
			ContactPoints:         []altinn.ContactPoint{{Category: "Kundesenter", Email: "post@kartverket.no"}},
			Keywords:              []altinn.Keyword{{Word: "matrikkel", Language: "nb"}},
		}}}))
		Expect(registry.policies).To(Equal([]string{"kv-matrikkel-brukernavn"}))
		Expect(registry.updated).To(BeEmpty())
		Expect(readyCondition()).To(PointTo(MatchFields(IgnoreExtras, Fields{
			"Status": Equal(metav1.ConditionTrue), "Reason": Equal("Synced"), "ObservedGeneration": Equal(int64(1)),
		})))
	})

	It("withdraws the resource in Altinn and lets the MaskinportenSchemaResource go", func() {
		markDeleting()

		_, err := reconcile()

		Expect(err).NotTo(HaveOccurred())
		Expect(registry.withdrawn).To(Equal([]string{"kv-matrikkel-brukernavn"}))
		Expect(isGone()).To(BeTrue())
	})

	It("keeps the MaskinportenSchemaResource and tries again when withdrawing fails", func() {
		markDeleting()
		registry.withdrawErr = errors.New("withdrawing resource kv-matrikkel-brukernavn in Altinn: responded 503")

		_, err := reconcile()

		Expect(err).To(MatchError(ContainSubstring("responded 503")))
		saved := new(altinnatorv1alpha1.MaskinportenSchemaResource)
		Expect(kubeClient.Get(context.Background(), key, saved)).To(Succeed())
		Expect(saved.Finalizers).To(ContainElement("altinnator.kartverket.no/withdraw"))
	})

	DescribeTable("lets the MaskinportenSchemaResource go when Altinn has nothing of ours to withdraw",
		func(withdrawErr error) {
			markDeleting()
			registry.withdrawErr = withdrawErr

			_, err := reconcile()

			Expect(err).NotTo(HaveOccurred())
			Expect(isGone()).To(BeTrue())
		},
		Entry("when the resource was never created",
			fmt.Errorf("resource kv-matrikkel-brukernavn: %w", altinn.ErrNotFound)),
		Entry("when another resource has the id",
			fmt.Errorf("resource kv-matrikkel-brukernavn: %w", altinn.ErrTaken)),
	)

	It("only withdraws a MaskinportenSchemaResource that is being deleted", func() {
		markDeleting()
		registry.inAltinn = sameAsSpec()
		registry.inAltinn.Title.Nb = "Gammel tittel"

		_, _ = reconcile()

		Expect(registry.created).To(BeEmpty())
		Expect(registry.updated).To(BeEmpty())
		Expect(registry.policies).To(BeEmpty())
	})

	It("does not withdraw anything in Altinn when the resource is not being deleted", func() {
		_, _ = reconcile()

		Expect(registry.withdrawn).To(BeEmpty())
	})

	DescribeTable("updates the resource in Altinn when it differs from the spec",
		func(change func(inAltinn *altinn.MaskinportenSchema)) {
			markReady()
			registry.inAltinn = sameAsSpec()
			change(registry.inAltinn)

			_, err := reconcile()

			Expect(err).NotTo(HaveOccurred())
			Expect(registry.updated).To(HaveLen(1))
			Expect(registry.updated[0].id).To(Equal("kv-matrikkel-brukernavn"))
			Expect(registry.updated[0].content.Title.Nb).To(Equal("Matrikkel API"))
			Expect(registry.created).To(BeEmpty())
			Expect(registry.policies).To(BeEmpty())
			Expect(readyCondition().Status).To(Equal(metav1.ConditionTrue))
		},
		Entry("in the scopes", func(s *altinn.MaskinportenSchema) { s.Scopes = append(s.Scopes, "kartverk:matrikkel:adresse") }),
		Entry("in the title", func(s *altinn.MaskinportenSchema) { s.Title.Nb = "Gammel tittel" }),
		Entry("in the description", func(s *altinn.MaskinportenSchema) { s.Description.En = "Old" }),
		Entry("in the delegation description", func(s *altinn.MaskinportenSchema) { s.DelegationDescription.Nn = "Gamal" }),
		Entry("in the homepage", func(s *altinn.MaskinportenSchema) { s.Homepage = "" }),
		Entry("in the contact points", func(s *altinn.MaskinportenSchema) { s.ContactPoints[0].Telephone = "08700" }),
		Entry("in the keywords", func(s *altinn.MaskinportenSchema) { s.Keywords = nil }),
	)

	It("changes nothing in Altinn when the resource matches the spec", func() {
		markReady()
		registry.inAltinn = sameAsSpec()

		_, err := reconcile()

		Expect(err).NotTo(HaveOccurred())
		Expect(registry.created).To(BeEmpty())
		Expect(registry.updated).To(BeEmpty())
		Expect(registry.policies).To(BeEmpty())
	})

	DescribeTable("checks Altinn again later when the resource is in sync",
		func(setUp func()) {
			setUp()

			result, err := reconcile()

			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeNumerically(">", 0))
		},
		Entry("after creating it", func() {}),
		Entry("after updating it", func() {
			markReady()
			registry.inAltinn = sameAsSpec()
			registry.inAltinn.Title.Nb = "Gammel tittel"
		}),
		Entry("when it already matches the spec", func() {
			markReady()
			registry.inAltinn = sameAsSpec()
		}),
	)

	It("corrects a change made directly in Altinn on the next check", func() {
		markReady()
		registry.inAltinn = sameAsSpec()
		_, err := reconcile()
		Expect(err).NotTo(HaveOccurred())
		Expect(registry.updated).To(BeEmpty())

		registry.inAltinn.Title.Nb = "Endret i Altinn"
		_, err = reconcile()

		Expect(err).NotTo(HaveOccurred())
		Expect(registry.updated).To(HaveLen(1))
		Expect(registry.updated[0].content.Title.Nb).To(Equal("Matrikkel API"))
	})

	It("sets the policy when the resource matches the spec but has not been ready", func() {
		registry.inAltinn = sameAsSpec()

		_, err := reconcile()

		Expect(err).NotTo(HaveOccurred())
		Expect(registry.created).To(BeEmpty())
		Expect(registry.updated).To(BeEmpty())
		Expect(registry.policies).To(Equal([]string{"kv-matrikkel-brukernavn"}))
		Expect(readyCondition().Status).To(Equal(metav1.ConditionTrue))
	})

	DescribeTable("shows in the status, without trying again, when Altinn will never take the resource",
		func(setUp func(), reason, message string) {
			setUp()

			result, err := reconcile()

			Expect(err).NotTo(HaveOccurred())
			Expect(result.IsZero()).To(BeTrue())
			Expect(readyCondition()).To(PointTo(MatchFields(IgnoreExtras, Fields{
				"Status":  Equal(metav1.ConditionFalse),
				"Reason":  Equal(reason),
				"Message": ContainSubstring(message),
			})))
		},
		Entry("when Altinn rejects a new resource", func() {
			registry.createErr = fmt.Errorf(`responded 400: {"errors":{"Title":["Missing title in nn"]}}: %w`, altinn.ErrRejected)
		}, "Rejected", "Missing title in nn"),
		Entry("when Altinn rejects a changed resource", func() {
			registry.inAltinn = sameAsSpec()
			registry.inAltinn.Title.Nb = "Gammel tittel"
			registry.updateErr = fmt.Errorf(`responded 400: {"errors":{"Title":["Missing title in nn"]}}: %w`, altinn.ErrRejected)
		}, "Rejected", "Missing title in nn"),
		Entry("when the resource is withdrawn in Altinn", func() {
			registry.inAltinn = sameAsSpec()
			registry.inAltinn.Status = "Withdrawn"
		}, "Withdrawn", "withdrawn"),
		Entry("when another resource has the id", func() {
			registry.getErr = fmt.Errorf("resource kv-matrikkel-brukernavn is a MaskinportenSchema owned by 974760223: %w",
				altinn.ErrTaken)
		}, "Taken", "owned by 974760223"),
	)

	DescribeTable("writes nothing to a resource withdrawn in Altinn",
		func(change func(inAltinn *altinn.MaskinportenSchema)) {
			registry.inAltinn = sameAsSpec()
			registry.inAltinn.Status = "Withdrawn"
			change(registry.inAltinn)

			_, err := reconcile()

			Expect(err).NotTo(HaveOccurred())
			Expect(registry.created).To(BeEmpty())
			Expect(registry.updated).To(BeEmpty())
			Expect(registry.policies).To(BeEmpty())
		},
		Entry("when it matches the spec", func(*altinn.MaskinportenSchema) {}),
		Entry("when it differs from the spec", func(s *altinn.MaskinportenSchema) { s.Title.Nb = "Gammel tittel" }),
	)

	It("does not set the policy when Altinn rejects the new resource", func() {
		registry.createErr = fmt.Errorf("responded 400: %w", altinn.ErrRejected)

		_, err := reconcile()

		Expect(err).NotTo(HaveOccurred())
		Expect(registry.policies).To(BeEmpty())
	})

	It("shows the error in the status and tries again when Altinn fails", func() {
		registry.createErr = errors.New("creating resource kv-matrikkel-brukernavn in Altinn: responded 503")

		_, err := reconcile()

		Expect(err).To(MatchError(ContainSubstring("responded 503")))
		Expect(readyCondition()).To(PointTo(MatchFields(IgnoreExtras, Fields{
			"Status":  Equal(metav1.ConditionFalse),
			"Reason":  Equal("Failed"),
			"Message": ContainSubstring("responded 503"),
		})))
	})

	It("does nothing when the MaskinportenSchemaResource is gone", func() {
		scheme := runtime.NewScheme()
		Expect(altinnatorv1alpha1.AddToScheme(scheme)).To(Succeed())
		reconciler := &controller.MaskinportenSchemaResourceReconciler{
			Client:   fake.NewClientBuilder().WithScheme(scheme).Build(),
			Registry: registry,
		}

		_, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: key})

		Expect(err).NotTo(HaveOccurred())
		Expect(registry.created).To(BeEmpty())
	})

	It("gets a new generation when deleted, so GenerationChangedPredicate lets the delete through", func() {
		created := resource.DeepCopy()
		created.Namespace = "default"
		created.Name = "delete-bumps-generation"
		created.Generation = 0
		created.ResourceVersion = ""
		created.Finalizers = []string{"altinnator.kartverket.no/withdraw"}
		Expect(k8sClient.Create(ctx, created)).To(Succeed())
		DeferCleanup(func() {
			latest := &altinnatorv1alpha1.MaskinportenSchemaResource{}
			if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(created), latest); err != nil {
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
				return
			}
			latest.Finalizers = nil
			Expect(client.IgnoreNotFound(k8sClient.Update(ctx, latest))).To(Succeed())
		})

		Expect(k8sClient.Delete(ctx, created)).To(Succeed())

		saved := &altinnatorv1alpha1.MaskinportenSchemaResource{}
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(created), saved)).To(Succeed())
		Expect(saved.DeletionTimestamp).NotTo(BeNil())
		Expect(saved.Generation).To(BeNumerically(">", created.Generation))
	})
})
