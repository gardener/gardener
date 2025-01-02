// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package hibernation

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/robfig/cron"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/component-base/version"
	testclock "k8s.io/utils/clock/testing"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
)

var _ = Describe("Shoot Hibernation", func() {
	type testEntry struct {
		timeNow                 func() time.Time
		triggerDeadlineDuration time.Duration
		shootSettings           func(shoot *gardencorev1beta1.Shoot)

		triggerHibernationOrWakeup  bool
		expectedRequeueDurationFunc func(now time.Time) time.Duration
		expectedHibernationEnabled  bool
	}

	var (
		everyDayAt2      = "00 02 * * 1,2,3,4,5,6,0"
		everyDayAt7      = "00 07 * * 1,2,3,4,5,6,0"
		everyWeekDayAt8  = "00 08 * * 1,2,3,4,5"
		everyWeekDayAt19 = "00 19 * * 1,2,3,4,5"

		locationEUBerlin = "Europe/Berlin"
		locationEUSofia  = "Europe/Sofia"

		weekDayAt2  = "2022-04-12T02:00:00Z"
		weekDayAt0  = "2022-04-12T00:00:00Z"
		weekDayAt7  = "2022-04-12T07:00:00Z"
		weekDayAt19 = "2022-04-12T19:00:00Z"

		noDeadLine    = -1234 * time.Second
		shortDeadline = 10 * time.Second
		longDeadline  = 10 * time.Hour

		mustParseStandard = func(standardSpec string) cron.Schedule {
			sched, err := cron.ParseStandard(standardSpec)
			Expect(err).NotTo(HaveOccurred())
			return sched
		}

		mustLoadLocation = func(locationName string) time.Location {
			location, err := time.LoadLocation(locationName)
			Expect(err).NotTo(HaveOccurred())
			return *location
		}

		requeueAfterBasedOnSchedule = func(schedule, location string) func(now time.Time) time.Duration {
			return func(now time.Time) time.Duration {
				parsedSchedule := mustParseStandard(schedule)
				loc := mustLoadLocation(location)
				return parsedSchedule.Next(now.In(&loc)).Add(nextScheduleDelta).Sub(now)
			}
		}

		mustParseRFC3339Time = func(t string) time.Time {
			parsedTime, err := time.Parse(time.RFC3339, t)
			Expect(err).NotTo(HaveOccurred())
			return parsedTime
		}

		timeWithOffset = func(t string, offset time.Duration) func() time.Time {
			return func() time.Time {
				parsedTime := mustParseRFC3339Time(t)
				return parsedTime.Add(offset)
			}
		}
	)

	Context("parsedHibernationSchedule", func() {
		Describe("#next", func() {
			It("should correctly return the next scheduling time from the parsed schedule", func() {
				now := mustParseRFC3339Time(weekDayAt2)
				expected := mustParseRFC3339Time(weekDayAt0).Add(24 * time.Hour)

				parsedSchedule := parsedHibernationSchedule{
					location: mustLoadLocation(locationEUBerlin),
					schedule: mustParseStandard(everyDayAt2),
				}
				Expect(parsedSchedule.next(now)).To(Equal(expected))
			})
		})

		Describe("#previous", func() {
			It("should correctly return the previous scheduling time from the parsed schedule if it is within the specified range", func() {
				now := mustParseRFC3339Time(weekDayAt2)
				from := now.Add(-2 * 24 * time.Hour)

				expected := mustParseRFC3339Time(weekDayAt0)
				parsedSchedule := parsedHibernationSchedule{
					location: mustLoadLocation(locationEUBerlin),
					schedule: mustParseStandard(everyDayAt2),
				}
				prev := parsedSchedule.previous(from, now)
				Expect(prev).NotTo(BeNil())
				Expect(*prev).To(Equal(expected))
			})

			It("should return nil if previous scheduling time was not in specified range", func() {
				now := mustParseRFC3339Time(weekDayAt2)
				from := now.Add(-1 * time.Hour)

				parsedSchedule := parsedHibernationSchedule{
					location: mustLoadLocation(locationEUBerlin),
					schedule: mustParseStandard(everyDayAt2),
				}
				prev := parsedSchedule.previous(from, now)
				Expect(prev).To(BeNil())
			})
		})
	})

	Context("Shoot hibernation reconciliation", func() {
		Describe("#Reconcile", func() {
			var (
				ctx       context.Context
				c         client.Client
				now       time.Time
				fakeClock *testclock.FakeClock

				shoot *gardencorev1beta1.Shoot
			)

			BeforeEach(func() {
				ctx = context.TODO()

				shoot = &gardencorev1beta1.Shoot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "bar",
						Namespace: "garden-foo",
					},
					Spec: gardencorev1beta1.ShootSpec{
						Hibernation: &gardencorev1beta1.Hibernation{},
					},
					Status: gardencorev1beta1.ShootStatus{},
				}

				c = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithStatusSubresource(&gardencorev1beta1.Shoot{}).Build()
			})

			DescribeTable("should properly enable or disable hibernation and requeue the shoot", func(t testEntry) {
				By("Set current time")
				timeNow := now
				if t.timeNow != nil {
					timeNow = t.timeNow()
				}
				fakeClock = testclock.NewFakeClock(timeNow)

				if t.shootSettings != nil {
					By("Configure shoot")
					t.shootSettings(shoot)
				}

				By("Create shoot")
				Expect(c.Create(ctx, shoot)).To(Succeed())

				By("Configure hibernation reconciler")
				config := controllermanagerconfigv1alpha1.ShootHibernationControllerConfiguration{}
				if t.triggerDeadlineDuration != noDeadLine {
					config.TriggerDeadlineDuration = &metav1.Duration{Duration: t.triggerDeadlineDuration}
				}

				reconciler := &Reconciler{
					Client:   c,
					Config:   config,
					Recorder: record.NewFakeRecorder(1),
					Clock:    fakeClock,
				}

				By("Reconcile shoot resource")
				var requeueAfter time.Duration
				if t.expectedRequeueDurationFunc != nil {
					requeueAfter = t.expectedRequeueDurationFunc(timeNow)
				}
				result, err := reconciler.Reconcile(ctx,
					reconcile.Request{
						NamespacedName: client.ObjectKeyFromObject(shoot),
					},
				)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{RequeueAfter: requeueAfter}))

				reconciledShoot := &gardencorev1beta1.Shoot{}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(shoot), reconciledShoot)).To(Succeed())
				if t.triggerHibernationOrWakeup {
					Expect(reconciledShoot.Spec.Hibernation.Enabled).To(PointTo(Equal(t.expectedHibernationEnabled)))
					Expect(reconciledShoot.Status.LastHibernationTriggerTime.Time.UTC()).To(Equal(timeNow))
				} else {
					Expect(reconciledShoot.Spec.Hibernation.Enabled).To(BeNil())
				}
			},
				Entry("when there are no hibernation schedules nothing should be done", testEntry{
					timeNow: timeWithOffset(weekDayAt2, 0),
					shootSettings: func(shoot *gardencorev1beta1.Shoot) {
						shoot.CreationTimestamp = metav1.Time{Time: timeWithOffset(weekDayAt2, 0)()}
					},
				}),
				Entry("when hibernation schedule is incorrect nothing should be done and shoot must not be requeued", testEntry{
					timeNow: timeWithOffset(weekDayAt2, 0),
					shootSettings: func(shoot *gardencorev1beta1.Shoot) {
						shoot.CreationTimestamp = metav1.Time{Time: timeWithOffset(weekDayAt2, 0)()}
						shoot.Spec.Hibernation.Schedules = []gardencorev1beta1.HibernationSchedule{{Start: ptr.To("")}}
					},
				}),
				Entry("when shoot has never been hibernated and reconciliation is executed 30 seconds before wakeup schedule", testEntry{
					timeNow: timeWithOffset(weekDayAt2, -30*time.Second),
					shootSettings: func(shoot *gardencorev1beta1.Shoot) {
						shoot.CreationTimestamp = metav1.Time{Time: timeWithOffset(weekDayAt2, -1*24*time.Hour)()}
						shoot.Spec.Hibernation.Schedules = []gardencorev1beta1.HibernationSchedule{{
							Start: &everyDayAt7,
							End:   &everyDayAt2,
						}}
					},
					triggerDeadlineDuration:     noDeadLine,
					triggerHibernationOrWakeup:  true,
					expectedHibernationEnabled:  true,
					expectedRequeueDurationFunc: requeueAfterBasedOnSchedule(everyDayAt2, "UTC"),
				}),
				Entry("when shoot has never been hibernated and reconciliation is executed just before hibernation schedule", testEntry{
					timeNow: timeWithOffset(weekDayAt7, -1*time.Second),
					shootSettings: func(shoot *gardencorev1beta1.Shoot) {
						shoot.CreationTimestamp = metav1.Time{Time: timeWithOffset(weekDayAt7, -1*24*time.Hour)()}
						shoot.Spec.Hibernation.Schedules = []gardencorev1beta1.HibernationSchedule{{
							Start: &everyDayAt7,
							End:   &everyDayAt2,
						}}
					},
					triggerDeadlineDuration:     noDeadLine,
					triggerHibernationOrWakeup:  true,
					expectedHibernationEnabled:  false,
					expectedRequeueDurationFunc: requeueAfterBasedOnSchedule(everyDayAt7, "UTC"),
				}),
				Entry("when shoot has never hibernated and reconciliation is executed just after hibernation start schedule", testEntry{
					timeNow: timeWithOffset(weekDayAt7, 1*time.Second),
					shootSettings: func(shoot *gardencorev1beta1.Shoot) {
						shoot.CreationTimestamp = metav1.Time{Time: timeWithOffset(weekDayAt7, -1*24*time.Hour)()}
						shoot.Spec.Hibernation.Schedules = []gardencorev1beta1.HibernationSchedule{{
							Start: &everyDayAt7,
							End:   &everyDayAt2,
						}}
					},
					triggerDeadlineDuration:     noDeadLine,
					triggerHibernationOrWakeup:  true,
					expectedHibernationEnabled:  true,
					expectedRequeueDurationFunc: requeueAfterBasedOnSchedule(everyDayAt2, "UTC"),
				}),
				Entry("when shoot has never been hibernated and reconciliation is executed exactly at hibernation start schedule", testEntry{
					timeNow: timeWithOffset(weekDayAt7, 0),
					shootSettings: func(shoot *gardencorev1beta1.Shoot) {
						shoot.CreationTimestamp = metav1.Time{Time: timeWithOffset(weekDayAt7, -30*time.Second)()}
						shoot.Spec.Hibernation.Schedules = []gardencorev1beta1.HibernationSchedule{{
							Start: &everyDayAt7,
							End:   &everyDayAt2,
						}}
					},
					triggerDeadlineDuration:     noDeadLine,
					triggerHibernationOrWakeup:  true,
					expectedHibernationEnabled:  true,
					expectedRequeueDurationFunc: requeueAfterBasedOnSchedule(everyDayAt2, "UTC"),
				}),
				Entry("when shoot has just been created", testEntry{
					timeNow: timeWithOffset(weekDayAt19, 1*time.Second),
					shootSettings: func(shoot *gardencorev1beta1.Shoot) {
						shoot.CreationTimestamp = metav1.Time{Time: timeWithOffset(weekDayAt19, 0)()}
						shoot.Spec.Hibernation.Schedules = []gardencorev1beta1.HibernationSchedule{{
							Start: &everyDayAt7,
							End:   &everyDayAt2,
						}}
					},
					triggerDeadlineDuration:     noDeadLine,
					expectedRequeueDurationFunc: requeueAfterBasedOnSchedule(everyDayAt2, "UTC"),
				}),
				Entry("when shoot has never been hibernated and has multiple hibernation schedules and reconciliation is executed just after hibernation", testEntry{
					timeNow: timeWithOffset(weekDayAt7, 1*time.Second),
					shootSettings: func(shoot *gardencorev1beta1.Shoot) {
						shoot.CreationTimestamp = metav1.Time{Time: timeWithOffset(weekDayAt7, -1*24*time.Hour)()}
						shoot.Spec.Hibernation.Schedules = []gardencorev1beta1.HibernationSchedule{
							{
								Start:    &everyDayAt7,
								End:      &everyDayAt2,
								Location: &locationEUBerlin,
							},
							{
								Start:    &everyWeekDayAt8,
								End:      &everyWeekDayAt19,
								Location: &locationEUSofia,
							},
							{
								Start: &everyDayAt2,
								End:   &everyDayAt7,
							},
						}
					},
					triggerDeadlineDuration:     noDeadLine,
					triggerHibernationOrWakeup:  true,
					expectedHibernationEnabled:  false,
					expectedRequeueDurationFunc: requeueAfterBasedOnSchedule(everyWeekDayAt19, locationEUSofia),
				}),
				Entry("when shoot has been hibernated or woken up previously and reconciliation is executed exactly after hibernation start time", testEntry{
					timeNow: timeWithOffset(weekDayAt7, 1*time.Second),
					shootSettings: func(shoot *gardencorev1beta1.Shoot) {
						shoot.CreationTimestamp = metav1.Time{Time: timeWithOffset(weekDayAt7, -24*time.Hour)()}
						shoot.Spec.Hibernation.Schedules = []gardencorev1beta1.HibernationSchedule{{
							Start: &everyDayAt7,
							End:   &everyDayAt2,
						}}
						shoot.Status.LastHibernationTriggerTime = &metav1.Time{Time: timeWithOffset(weekDayAt2, 0)()}
					},
					triggerDeadlineDuration:     noDeadLine,
					triggerHibernationOrWakeup:  true,
					expectedHibernationEnabled:  true,
					expectedRequeueDurationFunc: requeueAfterBasedOnSchedule(everyDayAt2, "UTC"),
				}),
				Entry("when shoot has been hibernated or woken up previously and reconciliation is executed before hibernation start time", testEntry{
					timeNow: timeWithOffset(weekDayAt7, -1*time.Second),
					shootSettings: func(shoot *gardencorev1beta1.Shoot) {
						shoot.CreationTimestamp = metav1.Time{Time: timeWithOffset(weekDayAt7, -24*time.Hour)()}
						shoot.Spec.Hibernation.Schedules = []gardencorev1beta1.HibernationSchedule{{
							Start: &everyDayAt7,
							End:   &everyDayAt2,
						}}
						shoot.Status.LastHibernationTriggerTime = &metav1.Time{Time: timeWithOffset(weekDayAt2, 0)()}
					},
					triggerDeadlineDuration:     noDeadLine,
					expectedRequeueDurationFunc: requeueAfterBasedOnSchedule(everyDayAt7, "UTC"),
				}),
				Entry("when shoot was not hibernated and current reconciliation is within the hibernation deadline", testEntry{
					timeNow: timeWithOffset(weekDayAt7, 1*time.Second),
					shootSettings: func(shoot *gardencorev1beta1.Shoot) {
						shoot.CreationTimestamp = metav1.Time{Time: timeWithOffset(weekDayAt7, -24*time.Hour)()}
						shoot.Spec.Hibernation.Schedules = []gardencorev1beta1.HibernationSchedule{{
							Start: &everyDayAt7,
							End:   &everyDayAt2,
						}}
					},
					triggerDeadlineDuration:     longDeadline,
					triggerHibernationOrWakeup:  true,
					expectedHibernationEnabled:  true,
					expectedRequeueDurationFunc: requeueAfterBasedOnSchedule(everyDayAt2, "UTC"),
				}),
				Entry("when shoot was previously hibernated and current reconciliation is within the hibernation deadline", testEntry{
					timeNow: timeWithOffset(weekDayAt7, 1*time.Second),
					shootSettings: func(shoot *gardencorev1beta1.Shoot) {
						shoot.CreationTimestamp = metav1.Time{Time: timeWithOffset(weekDayAt7, -24*time.Hour)()}
						shoot.Spec.Hibernation.Schedules = []gardencorev1beta1.HibernationSchedule{{
							Start: &everyDayAt7,
							End:   &everyDayAt2,
						}}
						shoot.Status.LastHibernationTriggerTime = &metav1.Time{Time: timeWithOffset(weekDayAt2, 0)()}
					},
					triggerDeadlineDuration:     longDeadline,
					triggerHibernationOrWakeup:  true,
					expectedHibernationEnabled:  true,
					expectedRequeueDurationFunc: requeueAfterBasedOnSchedule(everyDayAt2, "UTC"),
				}),
				Entry("when shoot was not previously hibernated and current reconciliation is outside hibernation deadline", testEntry{
					timeNow: timeWithOffset(weekDayAt7, shortDeadline+1*time.Second),
					shootSettings: func(shoot *gardencorev1beta1.Shoot) {
						shoot.CreationTimestamp = metav1.Time{Time: timeWithOffset(weekDayAt7, -24*time.Hour)()}
						shoot.Spec.Hibernation.Schedules = []gardencorev1beta1.HibernationSchedule{{
							Start: &everyDayAt7,
							End:   &everyDayAt2,
						}}
					},
					triggerDeadlineDuration:     shortDeadline,
					expectedRequeueDurationFunc: requeueAfterBasedOnSchedule(everyDayAt2, "UTC"),
				}),
				Entry("when shoot was previously hibernated and current reconciliation is outside hibernation deadline", testEntry{
					timeNow: timeWithOffset(weekDayAt7, shortDeadline+1*time.Second),
					shootSettings: func(shoot *gardencorev1beta1.Shoot) {
						shoot.CreationTimestamp = metav1.Time{Time: timeWithOffset(weekDayAt7, -24*time.Hour)()}
						shoot.Spec.Hibernation.Schedules = []gardencorev1beta1.HibernationSchedule{{
							Start: &everyDayAt7,
							End:   &everyDayAt2,
						}}
						shoot.Status.LastHibernationTriggerTime = &metav1.Time{Time: timeWithOffset(weekDayAt2, 0)()}
					},
					triggerDeadlineDuration:     shortDeadline,
					expectedRequeueDurationFunc: requeueAfterBasedOnSchedule(everyDayAt2, "UTC"),
				}),
				Entry("when shoot is in failed state", testEntry{
					timeNow: timeWithOffset(weekDayAt7, shortDeadline+1*time.Second),
					shootSettings: func(shoot *gardencorev1beta1.Shoot) {
						shoot.CreationTimestamp = metav1.Time{Time: timeWithOffset(weekDayAt7, 1*time.Second)()}
						shoot.Spec.Hibernation.Schedules = []gardencorev1beta1.HibernationSchedule{
							{
								Start: &everyDayAt7,
								End:   &everyDayAt2,
							},
						}
						shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateFailed}
						shoot.Status.Gardener.Version = version.Get().GitVersion
					},
					triggerDeadlineDuration:     shortDeadline,
					expectedRequeueDurationFunc: requeueAfterBasedOnSchedule(everyDayAt2, "UTC"),
				}),
			)
		})
	})
})
