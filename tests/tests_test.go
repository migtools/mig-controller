package tests

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/labels"
	"log"
	"path"
	"strings"
	"time"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/pods"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var ctx = context.Background()

var _ = Describe("Running migmigration when storage, cluster and plan are ready", func() {
	var (
		migPlan      *migapi.MigPlan
		migPlanName  string
		namespaces   []string
		testFilePath string
	)

	JustBeforeEach(func() {
		log.Println("creating migplan")
		Expect(hostClient.Create(ctx, migPlan)).Should(Succeed())
		Eventually(func() bool {
			log.Println("waiting for migPlan to be ready")
			Expect(hostClient.Get(ctx, client.ObjectKey{Name: migPlanName, Namespace: MigrationNamespace}, migPlan)).Should(Succeed())
			return migPlan.Status.IsReady()
		}, time.Minute*5, time.Second).Should(Equal(true))
	})

	AfterEach(func() {
		err = hostClient.Delete(ctx, &migapi.MigPlan{
			ObjectMeta: metav1.ObjectMeta{
				Name:      migPlanName,
				Namespace: MigrationNamespace,
			},
		})
		if err != nil {
			log.Println(err)
		}
	})

	Context("Testing BZ #1965421, PVC name is more than 63 char long", func() {

		BeforeEach(func() {
			migPlanName = "e2e-1965421"
			namespaces = []string{"ocp-41583-longpvcname"}
			testFilePath = "data/test/"
			migPlan = NewMigPlan(namespaces, migPlanName)

			_, err = sourceClient.CoreV1().Namespaces().Create(ctx, NewMigrationNS("ocp-41583-longpvcname"), metav1.CreateOptions{})
			if errors.IsAlreadyExists(err) {
				log.Println(err)
			} else {
				Expect(err).ToNot(HaveOccurred())
			}

			_, err = sourceClient.CoreV1().PersistentVolumeClaims("ocp-41583-longpvcname").Create(ctx, NewPVC("long-name-123456789011121314151617181920212223242526272829303132", "ocp-41583-longpvcname"), metav1.CreateOptions{})

			if errors.IsAlreadyExists(err) {
				log.Println(err)
			} else {
				Expect(err).ToNot(HaveOccurred())
			}

			_, err = sourceClient.AppsV1().Deployments("ocp-41583-longpvcname").Create(ctx, NewDeploymentFor41583(), metav1.CreateOptions{})
			if errors.IsAlreadyExists(err) {
				log.Println(err)
			} else {
				Expect(err).ToNot(HaveOccurred())
			}
			Eventually(func() v1.PodPhase {
				podList, err := sourceClient.CoreV1().Pods("ocp-41583-longpvcname").List(ctx, metav1.ListOptions{})
				Expect(err).ToNot(HaveOccurred())
				for _, p := range podList.Items {
					if p.Status.Phase == "Running" {
						podCmd := pods.PodCommand{
							Pod:     &p,
							RestCfg: sourceCfg,
							Args: []string {"sh", "-c", "/usr/local/bin/generate_sample.sh -d /data/test -m 10"},
						}
						err = podCmd.Run()
						if err != nil {
							log.Println(err, "Failed running ls command inside destination Pod",
								path.Join(p.Namespace, p.Name),
								"command", "/usr/local/bin/generate_sample.sh -d "+testFilePath+" -m 10")
						}
					}
					log.Println(fmt.Sprintf("Waiting for pod %s to be ready, currently in phase - %s", p.Name, p.Status.Phase))
					return p.Status.Phase
				}
				return ""
			}, time.Minute*5, time.Second).Should(Equal(v1.PodRunning))
			log.Println("app deployed on the source cluster")
		})

		AfterEach(func() {
			for _, ns := range namespaces {
				err = sourceClient.CoreV1().Namespaces().Delete(ctx, ns, metav1.DeleteOptions{})
				if err != nil {
					log.Println(err)
				}
				err = hostClient.Delete(context.TODO(), &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})
				if err != nil {
					log.Println(err)
				}
			}
		})

		It("Should create a new migration", func() {
			By("Creating a new migmigration")

			migrationName := "foo"
			migration := NewMigMigration(migrationName, migPlanName, false, false)
			Expect(hostClient.Create(ctx, migration)).Should(Succeed())

			fooMigration := &migapi.MigMigration{}

			Eventually(func() string {
				log.Println("waiting for dvm to complete")
				dvms := &migapi.DirectVolumeMigrationList{}
				Expect(hostClient.List(ctx, dvms, client.InNamespace(MigrationNamespace))).Should(Succeed())
				for _, dvm := range dvms.Items {
					if dvm.GenerateName == migrationName+"-" {
						if dvm.Status.Itinerary == "VolumeMigration" {
							dvmps := &migapi.DirectVolumeMigrationProgressList{}
							Expect(hostClient.List(ctx, dvmps, client.MatchingLabels{"directvolumemigration": string(dvm.UID)})).Should(Succeed())
							for _, dvmp := range dvmps.Items {
								Expect(dvmp.Status.HasCondition(migapi.ReconcileFailed)).ShouldNot(BeTrue())
								if dvmp.Status.TotalProgressPercentage == "100%" {
									return dvm.Status.Phase
								}
							}
						} else if dvm.Status.Itinerary == "MigrationFailed" {
							log.Println("Direct volume migration Failed")
							Expect(dvm.Status.Itinerary).Should(BeEquivalentTo("VolumeMigration"))
						}
					}
				}
				return ""
			}, time.Minute*15, time.Second).Should(Equal("Completed"))
			Eventually(func() string {
				Expect(hostClient.Get(ctx, client.ObjectKey{Name: migrationName, Namespace: MigrationNamespace}, fooMigration)).Should(Succeed())
				Expect(fooMigration.Status.Itinerary).Should(Equal("Final"))
				log.Println(fmt.Sprintf("Running migration, currenlty in phase - %s", fooMigration.Status.Phase))
				return fooMigration.Status.Phase
			}, time.Minute*5, time.Second).Should(Equal("Completed"))

			// TODO check the destination pvc for data
			Eventually(func() bool {
				log.Println("Verifying data on destination")
				podList := &v1.PodList{}
				err = hostClient.List(ctx, podList, &client.ListOptions{
					LabelSelector: labels.SelectorFromSet(map[string]string{
						"app": "longpvc-test",
					},
					),
					Namespace: "ocp-41583-longpvcname",
				})
				Expect(err).ToNot(HaveOccurred())
				destinationFile := []string{}
				for _, p := range podList.Items {
					podCmd := pods.PodCommand{
						Pod:     &p,
						RestCfg: hostCfg,
						Args:    []string{"sh", "-c", "cd " + testFilePath + "&& md5sum *"},
					}
					err = podCmd.Run()
					if err != nil {
						log.Println(err, "Failed running ls command inside destination Pod",
							"pod", path.Join(p.Namespace, p.Name),
							"command", "cd "+testFilePath+"&& md5sum *")
					}
					destinationFile = strings.Split(podCmd.Out.String(), " ")
					break
				}
				podList, err = sourceClient.CoreV1().Pods("ocp-41583-longpvcname").List(ctx, metav1.ListOptions{
					LabelSelector: labels.SelectorFromSet(map[string]string{
						"app": "longpvc-test",
					},
					).String(),
				})
				Expect(err).ToNot(HaveOccurred())
				for _, p := range podList.Items {
					podCmd := pods.PodCommand{
						Pod:     &p,
						RestCfg: sourceCfg,
						Args:    []string{"sh", "-c", "cd " + testFilePath+"&& md5sum *"},
					}
					err = podCmd.Run()
					if err != nil {
						log.Println(err, "Failed running ls command inside source Pod",
							"pod", path.Join(p.Namespace, p.Name),
							"command", "cd "+testFilePath+"&& md5sum *")
					}
					sourceFile := strings.Split(podCmd.Out.String(), " ")
					if len(destinationFile) == 1 {
						return false
					}
					for i, v := range destinationFile {
						Expect(v).Should(BeEquivalentTo(sourceFile[i]))
					}
					return true
				}
				return false
			}, time.Minute*5, time.Second).Should(Equal(true))
		})
	})
})
