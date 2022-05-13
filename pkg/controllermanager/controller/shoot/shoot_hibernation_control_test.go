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
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/shoot"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/robfig/cron"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Shoot Hibernation", func() {
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
	)

	Context("ParsedHibernationSchedule", func() {
		Describe("#Next", func() {
			It("should correctly return the next scheduling time from the parsed schedule", func() {
				now := mustParseRFC3339Time(weekDayAt2)
				expected := mustParseRFC3339Time(weekDayAt0).Add(24 * time.Hour)

				parsedSchedule := ParsedHibernationSchedule{
					Location: mustLoadLocation(locationEUBerlin),
					Schedule: mustParseStandard(everyDayAt2),
				}
				Expect(parsedSchedule.Next(now)).To(Equal(expected))
			})
		})

		Describe("#Prev", func() {
			It("should correctly return the previous scheduling time from the parsed schedule if it is within the specified range", func() {
				now := mustParseRFC3339Time(weekDayAt2)
				from := now.Add(-2 * 24 * time.Hour)

				expected := mustParseRFC3339Time(weekDayAt0)
				parsedSchedule := ParsedHibernationSchedule{
					Location: mustLoadLocation(locationEUBerlin),
					Schedule: mustParseStandard(everyDayAt2),
				}
				prev := parsedSchedule.Prev(from, now)
				Expect(prev).NotTo(BeNil())
				Expect(*prev).To(Equal(expected))
			})

			It("should return nil if previous scheduling time was not in specified range", func() {
				now := mustParseRFC3339Time(weekDayAt2)
				from := now.Add(-1 * time.Hour)

				parsedSchedule := ParsedHibernationSchedule{
					Location: mustLoadLocation(locationEUBerlin),
					Schedule: mustParseStandard(everyDayAt2),
				}
				prev := parsedSchedule.Prev(from, now)
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
				fakeClock *clock.FakeClock

				shoot *gardencorev1beta1.Shoot
			)

			BeforeEach(func() {
				ctx = context.TODO()
				c = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()

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
			})

			DescribeTable("should properly enable or disable hibernation and requeue the shoot", func(t testEntry) {
				By("setting current time")
				timeNow := now
				if t.timeNow != nil {
					timeNow = t.timeNow()
				}
				fakeClock = clock.NewFakeClock(timeNow)

				By("configuring shoot")
				shootCreationTimestamp := now
				if t.shootCreationTime != nil {
					shootCreationTimestamp = t.shootCreationTime()
				}
				shoot.CreationTimestamp = metav1.Time{Time: shootCreationTimestamp}
				if t.lastHibernationTriggerTime != nil {
					shoot.Status.LastHibernationTriggerTime = &metav1.Time{Time: t.lastHibernationTriggerTime()}
				}
				shoot.Spec.Hibernation.Schedules = t.schedules

				By("creating shoot")
				Expect(c.Create(ctx, shoot)).To(Succeed())

				By("configure hibernation reconciler")
				var requeueAfter time.Duration
				if t.expectedRequeueDurationFunc != nil {
					requeueAfter = t.expectedRequeueDurationFunc(timeNow)
				}
				config := config.ShootHibernationControllerConfiguration{}
				if t.triggerDeadlineDuration != noDeadLine {
					config.TriggerDeadlineDuration = &metav1.Duration{Duration: t.triggerDeadlineDuration}
				}
				reconciler := NewShootHibernationReconciler(c, config, record.NewFakeRecorder(1), fakeClock)

				By("reconciling shoot resource")
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
					Expect(reconciledShoot.Spec.Hibernation.Enabled).To(PointTo(Equal(t.hibernate)))
					Expect(reconciledShoot.Status.LastHibernationTriggerTime.Time.UTC()).To(Equal(timeNow))
				} else {
					Expect(reconciledShoot.Spec.Hibernation.Enabled).To(BeNil())
					if t.lastHibernationTriggerTime != nil {
						Expect(reconciledShoot.Status.LastHibernationTriggerTime.Time.UTC()).To(Equal(t.lastHibernationTriggerTime()))
					}
				}
			},
				Entry("when there are no hibernation schedules nothing should be done", testEntry{
					timeNow:           timeWithOffset(weekDayAt2, -30*time.Second),
					shootCreationTime: timeWithOffset(weekDayAt2, -1*24*time.Hour),
				}),
				Entry("when hibernation schedule is incorrect nothing should be done and shoot must not be requeued", testEntry{
					timeNow:           timeWithOffset(weekDayAt2, -30*time.Second),
					shootCreationTime: timeWithOffset(weekDayAt2, -1*24*time.Hour),
					schedules: []gardencorev1beta1.HibernationSchedule{
						{
							Start: pointer.String(""),
						},
					},
				}),
				Entry("when shoot has never been hibernated and reconciliation is executed 30 seconds before wakeup schedule", testEntry{
					timeNow:           timeWithOffset(weekDayAt2, -30*time.Second),
					shootCreationTime: timeWithOffset(weekDayAt2, -1*24*time.Hour),
					schedules: []gardencorev1beta1.HibernationSchedule{
						{
							Start: &everyDayAt7,
							End:   &everyDayAt2,
						},
					},
					triggerDeadlineDuration:     noDeadLine,
					triggerHibernationOrWakeup:  true,
					hibernate:                   true,
					expectedRequeueDurationFunc: requeueAfterBasedOnSchedule(everyDayAt2, "UTC"),
				}),
				Entry("when shoot has never been hibernated and reconciliation is executed just before hibernation schedule", testEntry{
					timeNow:           timeWithOffset(weekDayAt7, -1*time.Second),
					shootCreationTime: timeWithOffset(weekDayAt7, -1*24*time.Hour),
					schedules: []gardencorev1beta1.HibernationSchedule{
						{
							Start: &everyDayAt7,
							End:   &everyDayAt2,
						},
					},
					triggerDeadlineDuration:     noDeadLine,
					triggerHibernationOrWakeup:  true,
					hibernate:                   false,
					expectedRequeueDurationFunc: requeueAfterBasedOnSchedule(everyDayAt7, "UTC"),
				}),
				Entry("when shoot has never hibernated and reconciliation is executed just after hibernation start schedule", testEntry{
					timeNow:           timeWithOffset(weekDayAt7, 1*time.Second),
					shootCreationTime: timeWithOffset(weekDayAt7, -1*24*time.Hour),
					schedules: []gardencorev1beta1.HibernationSchedule{
						{
							Start: &everyDayAt7,
							End:   &everyDayAt2,
						},
					},
					triggerDeadlineDuration:     noDeadLine,
					triggerHibernationOrWakeup:  true,
					hibernate:                   true,
					expectedRequeueDurationFunc: requeueAfterBasedOnSchedule(everyDayAt2, "UTC"),
				}),
				Entry("when shoot has never been hibernated and reconciliation is executed exactly at hibernation start schedule", testEntry{
					timeNow:           timeWithOffset(weekDayAt7, 0),
					shootCreationTime: timeWithOffset(weekDayAt7, -30*time.Second),
					schedules: []gardencorev1beta1.HibernationSchedule{
						{
							Start: &everyDayAt7,
							End:   &everyDayAt2,
						},
					},
					triggerDeadlineDuration:     noDeadLine,
					triggerHibernationOrWakeup:  true,
					hibernate:                   true,
					expectedRequeueDurationFunc: requeueAfterBasedOnSchedule(everyDayAt2, "UTC"),
				}),
				Entry("when shoot has just been created", testEntry{
					timeNow:           timeWithOffset(weekDayAt19, 1*time.Second),
					shootCreationTime: timeWithOffset(weekDayAt19, 0),
					schedules: []gardencorev1beta1.HibernationSchedule{
						{
							Start: &everyDayAt7,
							End:   &everyDayAt2,
						},
					},
					triggerDeadlineDuration:     noDeadLine,
					expectedRequeueDurationFunc: requeueAfterBasedOnSchedule(everyDayAt2, "UTC"),
				}),
				Entry("when shoot has never been hibernated and has multiple hibernation schedules and reconciliation is executed just after hibernation", testEntry{
					timeNow:           timeWithOffset(weekDayAt7, 1*time.Second),
					shootCreationTime: timeWithOffset(weekDayAt7, -1*24*time.Hour),
					schedules: []gardencorev1beta1.HibernationSchedule{
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
					},
					triggerDeadlineDuration:     noDeadLine,
					triggerHibernationOrWakeup:  true,
					hibernate:                   false,
					expectedRequeueDurationFunc: requeueAfterBasedOnSchedule(everyWeekDayAt19, locationEUSofia),
				}),
				Entry("when shoot has been hibernated or woken up previously and reconciliation is executed exactly after hibernation start time", testEntry{
					timeNow:                    timeWithOffset(weekDayAt7, 1*time.Second),
					shootCreationTime:          timeWithOffset(weekDayAt7, -24*time.Hour),
					lastHibernationTriggerTime: timeWithOffset(weekDayAt2, 0),
					schedules: []gardencorev1beta1.HibernationSchedule{
						{
							Start: &everyDayAt7,
							End:   &everyDayAt2,
						},
					},
					triggerDeadlineDuration:     noDeadLine,
					triggerHibernationOrWakeup:  true,
					hibernate:                   true,
					expectedRequeueDurationFunc: requeueAfterBasedOnSchedule(everyDayAt2, "UTC"),
				}),
				Entry("when shoot has been hibernated or woken up previously and reconciliation is executed before hibernation start time", testEntry{
					timeNow:                    timeWithOffset(weekDayAt7, -1*time.Second),
					shootCreationTime:          timeWithOffset(weekDayAt7, -24*time.Hour),
					lastHibernationTriggerTime: timeWithOffset(weekDayAt2, 0),
					schedules: []gardencorev1beta1.HibernationSchedule{
						{
							Start: &everyDayAt7,
							End:   &everyDayAt2,
						},
					},
					triggerDeadlineDuration:     noDeadLine,
					expectedRequeueDurationFunc: requeueAfterBasedOnSchedule(everyDayAt7, "UTC"),
				}),
				Entry("when shoot was not hibernated and current reconciliation is within the hibernation deadline", testEntry{
					timeNow:           timeWithOffset(weekDayAt7, 1*time.Second),
					shootCreationTime: timeWithOffset(weekDayAt7, -24*time.Hour),
					schedules: []gardencorev1beta1.HibernationSchedule{
						{
							Start: &everyDayAt7,
							End:   &everyDayAt2,
						},
					},
					triggerDeadlineDuration:     longDeadline,
					triggerHibernationOrWakeup:  true,
					hibernate:                   true,
					expectedRequeueDurationFunc: requeueAfterBasedOnSchedule(everyDayAt2, "UTC"),
				}),
				Entry("when shoot was previously hibernated and current reconciliation is within the hibernation deadline", testEntry{
					timeNow:                    timeWithOffset(weekDayAt7, 1*time.Second),
					shootCreationTime:          timeWithOffset(weekDayAt7, -24*time.Hour),
					lastHibernationTriggerTime: timeWithOffset(weekDayAt2, 0),
					schedules: []gardencorev1beta1.HibernationSchedule{
						{
							Start: &everyDayAt7,
							End:   &everyDayAt2,
						},
					},
					triggerDeadlineDuration:     longDeadline,
					triggerHibernationOrWakeup:  true,
					hibernate:                   true,
					expectedRequeueDurationFunc: requeueAfterBasedOnSchedule(everyDayAt2, "UTC"),
				}),
				Entry("when shoot was not previously hibernated and current reconciliation is outside hibernation deadline", testEntry{
					timeNow:           timeWithOffset(weekDayAt7, shortDeadline+1*time.Second),
					shootCreationTime: timeWithOffset(weekDayAt7, -24*time.Hour),
					schedules: []gardencorev1beta1.HibernationSchedule{
						{
							Start: &everyDayAt7,
							End:   &everyDayAt2,
						},
					},
					triggerDeadlineDuration:     shortDeadline,
					expectedRequeueDurationFunc: requeueAfterBasedOnSchedule(everyDayAt2, "UTC"),
				}),
				Entry("when shoot was previously hibernated and current reconciliation is outside hibernation deadline", testEntry{
					timeNow:                    timeWithOffset(weekDayAt7, shortDeadline+1*time.Second),
					shootCreationTime:          timeWithOffset(weekDayAt7, -24*time.Hour),
					lastHibernationTriggerTime: timeWithOffset(weekDayAt2, 0),
					schedules: []gardencorev1beta1.HibernationSchedule{
						{
							Start: &everyDayAt7,
							End:   &everyDayAt2,
						},
					},
					triggerDeadlineDuration:     shortDeadline,
					expectedRequeueDurationFunc: requeueAfterBasedOnSchedule(everyDayAt2, "UTC"),
				}),
			)
		})
	})
})

type testEntry struct {
	timeNow                    func() time.Time
	shootCreationTime          func() time.Time
	lastHibernationTriggerTime func() time.Time
	schedules                  []gardencorev1beta1.HibernationSchedule
	triggerDeadlineDuration    time.Duration

	triggerHibernationOrWakeup  bool
	expectedRequeueDurationFunc func(now time.Time) time.Duration
	hibernate                   bool
}

func requeueAfterBasedOnSchedule(schedule, location string) func(now time.Time) time.Duration {
	return func(now time.Time) time.Duration {
		parsedSchedule := mustParseStandard(schedule)
		loc := mustLoadLocation(location)
		return parsedSchedule.Next(now.In(loc)).Sub(now)
	}
}

func timeWithOffset(t string, offset time.Duration) func() time.Time {
	return func() time.Time {
		parsedTime := mustParseRFC3339Time(t)
		return parsedTime.Add(offset)
	}
}

func mustParseRFC3339Time(t string) time.Time {
	parsedTime, err := time.Parse(time.RFC3339, t)
	Expect(err).NotTo(HaveOccurred())
	return parsedTime
}

func mustParseStandard(standardSpec string) cron.Schedule {
	sched, err := cron.ParseStandard(standardSpec)
	Expect(err).NotTo(HaveOccurred())
	return sched
}

func mustLoadLocation(locationName string) *time.Location {
	location, err := time.LoadLocation(locationName)
	Expect(err).NotTo(HaveOccurred())
	return location
}
