package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	storagev1alpha1 "github.com/cheap-man-ha-store/cheap-man-ha-store/api/v1alpha1"
)

var _ = Describe("PVCBackup Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		pvcBackup := &storagev1alpha1.PVCBackup{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind PVCBackup")
			err := k8sClient.Get(ctx, typeNamespacedName, pvcBackup)
			if err != nil && errors.IsNotFound(err) {
				resource := &storagev1alpha1.PVCBackup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: storagev1alpha1.PVCBackupSpec{
						BackupTargets: []storagev1alpha1.BackupTarget{
							{
								Name:     "test-s3",
								Type:     "s3",
								Priority: 1,
								S3: &storagev1alpha1.S3Config{
									Bucket: "test-bucket",
									Region: "us-west-2",
								},
							},
						},
						PVCSelector: storagev1alpha1.PVCSelector{
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app": "test",
								},
							},
						},
						Schedule: storagev1alpha1.BackupSchedule{
							Cron: "0 0 * * *", // Daily at midnight
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &storagev1alpha1.PVCBackup{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance PVCBackup")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := NewPVCBackupReconciler(k8sClient, k8sClient.Scheme())

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})
})
