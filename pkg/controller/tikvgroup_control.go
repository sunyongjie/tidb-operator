// Copyright 2020 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package controller

import (
	"fmt"

	"github.com/pingcap/tidb-operator/pkg/apis/pingcap/v1alpha1"
	"github.com/pingcap/tidb-operator/pkg/client/clientset/versioned"
	listers "github.com/pingcap/tidb-operator/pkg/client/listers/pingcap/v1alpha1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"
)

type TiKVGroupControlInterface interface {
	UpdateTiKVGroup(*v1alpha1.TiKVGroup, *v1alpha1.TiKVGroupStatus, *v1alpha1.TiKVGroupStatus) (*v1alpha1.TiKVGroup, error)
}

// NewRealTidbClusterControl creates a new TidbClusterControlInterface
func NewRealTiKVGroupControl(cli versioned.Interface,
	tgLister listers.TiKVGroupLister,
	recorder record.EventRecorder) TiKVGroupControlInterface {
	return &realTiKVGroupControl{
		cli,
		tgLister,
		recorder,
	}
}

type realTiKVGroupControl struct {
	cli      versioned.Interface
	tgLister listers.TiKVGroupLister
	recorder record.EventRecorder
}

func (rtc *realTiKVGroupControl) UpdateTiKVGroup(tg *v1alpha1.TiKVGroup, newStatus *v1alpha1.TiKVGroupStatus, oldStatus *v1alpha1.TiKVGroupStatus) (*v1alpha1.TiKVGroup, error) {
	ns := tg.GetNamespace()
	name := tg.GetName()

	status := tg.Status.DeepCopy()
	var updateTg *v1alpha1.TiKVGroup

	// don't wait due to limited number of clients, but backoff after the default number of steps
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var updateErr error
		updateTg, updateErr = rtc.cli.PingcapV1alpha1().TiKVGroups(ns).Update(tg)
		if updateErr == nil {
			klog.Infof("TiKVGroup: [%s/%s] updated successfully", ns, name)
			return nil
		}
		klog.Warningf("failed to update TiKVGroup: [%s/%s], error: %v", ns, name, updateErr)

		if updated, err := rtc.tgLister.TiKVGroups(ns).Get(name); err == nil {
			// make a copy so we don't mutate the shared cache
			tg = updated.DeepCopy()
			tg.Status = *status
		} else {
			utilruntime.HandleError(fmt.Errorf("error getting updated TiKVGroup %s/%s from lister: %v", ns, name, err))
		}

		return updateErr
	})
	return updateTg, err
}
