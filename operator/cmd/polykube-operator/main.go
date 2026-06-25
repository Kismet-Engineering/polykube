package main

import (
	"flag"
	"os"

	"github.com/Kismet-Engineering/polykube/operator/internal/controller"
	polykubescheme "github.com/Kismet-Engineering/polykube/operator/internal/scheme"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(polykubescheme.AddToScheme(scheme))
}

func main() {
	var metricsAddr string
	var probeAddr string
	var clusterMemberName string
	var enableLeaderElection bool

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.StringVar(&clusterMemberName, "cluster-member-name", "", "The ClusterMember resource name representing this cluster.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false, "Enable leader election for controller manager.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	log := ctrl.Log.WithName("setup")

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "polykube-operator.polykube.dev",
	})
	if err != nil {
		log.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err := (&controller.WorkloadReconciler{
		Client:            mgr.GetClient(),
		Scheme:            mgr.GetScheme(),
		ClusterMemberName: clusterMemberName,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create Workload controller")
		os.Exit(1)
	}

	if err := (&controller.ClusterMemberReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create ClusterMember controller")
		os.Exit(1)
	}

	if err := (&controller.FederationReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create Federation controller")
		os.Exit(1)
	}

	if err := (&controller.ServiceEndpointReconciler{
		Client:            mgr.GetClient(),
		Scheme:            mgr.GetScheme(),
		ClusterMemberName: clusterMemberName,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create ServiceEndpoint controller")
		os.Exit(1)
	}

	if err := (&controller.DatastoreBindingReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create DatastoreBinding controller")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		log.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		log.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	log.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Error(err, "manager exited")
		os.Exit(1)
	}
}
