package main

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)


func podsCount(){
	pods, err := k8sClientSet.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		logger.Panic("error getting pod count: ", err.Error())
	}

	logger.Info("Total pod running in cluster: ", len(pods.Items))

}
