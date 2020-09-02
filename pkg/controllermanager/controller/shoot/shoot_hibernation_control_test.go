// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package shoot_test

import (
	"context"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	fakeclientmap "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	fakeclientset "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/shoot"
	"github.com/gardener/gardener/pkg/logger"
	mockevent "github.com/gardener/gardener/pkg/mock/client-go/tools/record"
	mockgardencore "github.com/gardener/gardener/pkg/mock/gardener/client/core/clientset/versioned"
	mockgardencorev1beta1 "github.com/gardener/gardener/pkg/mock/gardener/client/core/clientset/versioned/typed/core/v1beta1"
	mockshoot "github.com/gardener/gardener/pkg/mock/gardener/controllermanager/controller/shoot"
	mocktime "github.com/gardener/gardener/pkg/mock/go/time"
	"github.com/gardener/gardener/pkg/utils/test"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/robfig/cron"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
)

// MustParseStandard parses the standardSpec and errors otherwise.
func MustParseStandard(standardSpec string) cron.Schedule {
	sched, err := cron.ParseStandard(standardSpec)
	Expect(err).NotTo(HaveOccurred())
	return sched
}

var _ = Describe("Shoot Hibernation", func() {
	var (
		ctrl *gomock.Controller
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	trueVar := true
	Context("HibernationSchedule", func() {
		Describe("#GroupHibernationSchedulesByLocation", func() {
			It("should group the hibernation schedules with the same location together", func() {
				var (
					locationEuropeBerlin = "Europe/Berlin"
					locationUSCentral    = "US/Central"

					s1 = gardencorev1beta1.HibernationSchedule{Location: &locationEuropeBerlin}
					s2 = gardencorev1beta1.HibernationSchedule{Location: &locationEuropeBerlin}
					s3 = gardencorev1beta1.HibernationSchedule{Location: &locationUSCentral}
					s4 = gardencorev1beta1.HibernationSchedule{}
				)

				grouped := GroupHibernationSchedulesByLocation([]gardencorev1beta1.HibernationSchedule{s1, s2, s3, s4})
				Expect(grouped).To(Equal(map[string][]gardencorev1beta1.HibernationSchedule{
					locationEuropeBerlin: {s1, s2},
					locationUSCentral:    {s3},
					time.UTC.String():    {s4},
				}))
			})
		})

		Describe("#ComputeHibernationSchedule", func() {
			It("should compute a correct hibernation schedule", func() {
				var (
					clientMap = fakeclientmap.NewClientMap()

					log      = logger.NewNopLogger()
					recorder = &record.FakeRecorder{}
					now      time.Time

					start = "0 * * * *"
					end   = "10 * * * *"

					startSched     = MustParseStandard(start)
					endSched       = MustParseStandard(end)
					location       = time.UTC
					locationString = location.String()

					shoot = gardencorev1beta1.Shoot{
						Spec: gardencorev1beta1.ShootSpec{
							Hibernation: &gardencorev1beta1.Hibernation{
								Enabled: &trueVar,
								Schedules: []gardencorev1beta1.HibernationSchedule{
									{
										Start:    &start,
										End:      &end,
										Location: &locationString,
									},
								},
							},
						},
					}

					timeNow             = mocktime.NewMockNow(ctrl)
					newCronWithLocation = mockshoot.NewMockNewCronWithLocation(ctrl)
					cr                  = mockshoot.NewMockCron(ctrl)
				)

				defer test.WithVars(
					&NewCronWithLocation, newCronWithLocation.Do,
					&TimeNow, timeNow.Do,
				)()

				timeNow.EXPECT().Do().Return(now).AnyTimes()

				gomock.InOrder(
					newCronWithLocation.EXPECT().Do(location).Return(cr),

					cr.EXPECT().Schedule(startSched, NewHibernationJob(clientMap, LocationLogger(log, location), recorder, &shoot, trueVar)),
					cr.EXPECT().Schedule(endSched, NewHibernationJob(clientMap, LocationLogger(log, location), recorder, &shoot, false)),
				)

				actualSched, err := ComputeHibernationSchedule(clientMap, log, recorder, &shoot)
				Expect(err).NotTo(HaveOccurred())
				Expect(actualSched).To(Equal(HibernationSchedule{locationString: cr}))
			})
		})

		Describe("#Start", func() {
			It("should start all crons", func() {
				var (
					c1 = mockshoot.NewMockCron(ctrl)
					c2 = mockshoot.NewMockCron(ctrl)

					sched = HibernationSchedule{"l1": c1, "l2": c2}
				)

				c1.EXPECT().Start()
				c2.EXPECT().Start()

				sched.Start()
			})
		})

		Describe("#Stop", func() {
			It("should stop all crons", func() {
				var (
					c1 = mockshoot.NewMockCron(ctrl)
					c2 = mockshoot.NewMockCron(ctrl)

					sched = HibernationSchedule{"l1": c1, "l2": c2}
				)

				c1.EXPECT().Stop()
				c2.EXPECT().Stop()

				sched.Stop()
			})
		})
	})

	Context("#HibernationScheduleRegistry", func() {
		var (
			k1, k2, k3 string

			s1, s2 HibernationSchedule

			reg HibernationScheduleRegistry
		)

		BeforeEach(func() {
			k1 = "foo"
			k2 = "bar"
			k3 = "baz"

			s1 = HibernationSchedule{k1: nil}
			s2 = HibernationSchedule{k2: nil}

			reg = NewHibernationScheduleRegistry()
		})

		Describe("#Load", func() {
			It("should correctly retrieve the entries", func() {
				reg.Store(k1, s1)
				reg.Store(k2, s2)

				actualS1, ok := reg.Load(k1)
				Expect(ok).To(BeTrue())
				Expect(actualS1).To(Equal(s1))

				actualS2, ok := reg.Load(k2)
				Expect(ok).To(BeTrue())
				Expect(actualS2).To(Equal(s2))

				_, ok = reg.Load(k3)
				Expect(ok).To(BeFalse())
			})
		})

		Describe("#Delete", func() {
			It("should delete the specified entry", func() {
				reg.Store(k1, s1)
				reg.Store(k2, s2)

				reg.Delete(k1)

				_, ok := reg.Load(k1)
				Expect(ok).To(BeFalse())

				actualS2, ok := reg.Load(k2)
				Expect(ok).To(BeTrue())
				Expect(actualS2).To(Equal(s2))
			})
		})
	})

	Context("HibernationJob", func() {
		Describe("#Run", func() {
			var (
				c           *mockgardencore.MockInterface
				gardenIface *mockgardencorev1beta1.MockCoreV1beta1Interface
				shootIface  *mockgardencorev1beta1.MockShootInterface
				clientMap   *fakeclientmap.ClientMap
				log         *logrus.Logger
				recorder    *mockevent.MockEventRecorder
				shoot       gardencorev1beta1.Shoot

				namespace string
				name      string
			)

			BeforeEach(func() {
				c = mockgardencore.NewMockInterface(ctrl)
				gardenIface = mockgardencorev1beta1.NewMockCoreV1beta1Interface(ctrl)
				shootIface = mockgardencorev1beta1.NewMockShootInterface(ctrl)
				clientMap = fakeclientmap.NewClientMap().AddClient(keys.ForGarden(), fakeclientset.NewClientSetBuilder().WithGardenCore(c).Build())
				log = logger.NewNopLogger()
				recorder = mockevent.NewMockEventRecorder(ctrl)

				namespace = "foo"
				name = "bar"
				shoot = gardencorev1beta1.Shoot{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "foo",
						Name:      "bar",
					},
					Spec: gardencorev1beta1.ShootSpec{
						Hibernation: &gardencorev1beta1.Hibernation{},
					},
				}
			})
			It("should set the hibernation status correctly to enabled", func() {
				enabled := true
				job := NewHibernationJob(clientMap, log, recorder, &shoot, enabled)

				gomock.InOrder(
					c.EXPECT().CoreV1beta1().Return(gardenIface),
					gardenIface.EXPECT().Shoots(namespace).Return(shootIface),
					shootIface.EXPECT().Get(context.Background(), name, metav1.GetOptions{}).Return(&shoot, nil),

					c.EXPECT().CoreV1beta1().Return(gardenIface),
					gardenIface.EXPECT().Shoots(namespace).Return(shootIface),
					shootIface.EXPECT().Update(context.Background(), gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{}), metav1.UpdateOptions{}).Do(func(_ context.Context, actual *gardencorev1beta1.Shoot, _ metav1.UpdateOptions) {
						Expect(actual.Spec.Hibernation).To(Equal(&gardencorev1beta1.Hibernation{
							Enabled: &enabled,
						}))
					}),
					recorder.EXPECT().Eventf(&shoot, corev1.EventTypeNormal, gardencorev1beta1.ShootEventHibernationEnabled, "%s", "Hibernating cluster due to schedule"),
				)

				job.Run()
			})

			It("should set the hibernation status correctly to disabled", func() {
				enabled := false
				job := NewHibernationJob(clientMap, log, recorder, &shoot, enabled)

				gomock.InOrder(c.EXPECT().CoreV1beta1().Return(gardenIface),
					gardenIface.EXPECT().Shoots(namespace).Return(shootIface),
					shootIface.EXPECT().Get(context.Background(), name, metav1.GetOptions{}).Return(&shoot, nil),

					c.EXPECT().CoreV1beta1().Return(gardenIface),
					gardenIface.EXPECT().Shoots(namespace).Return(shootIface),
					shootIface.EXPECT().Update(context.Background(), gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{}), metav1.UpdateOptions{}).Do(func(_ context.Context, actual *gardencorev1beta1.Shoot, _ metav1.UpdateOptions) {
						Expect(actual.Spec.Hibernation).To(Equal(&gardencorev1beta1.Hibernation{
							Enabled: &enabled,
						}))
					}),
					recorder.EXPECT().Eventf(&shoot, corev1.EventTypeNormal, gardencorev1beta1.ShootEventHibernationDisabled, "%s", "Waking up cluster due to schedule").Times(1),
				)

				job.Run()
			})
		})
	})
})
