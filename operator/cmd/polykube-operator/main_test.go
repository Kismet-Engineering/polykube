package main

import "testing"

func TestManagerOptionsWatchNamespace(t *testing.T) {
	t.Run("all namespaces by default", func(t *testing.T) {
		options := managerOptions(":8080", ":8081", "", true)

		if options.Cache.DefaultNamespaces != nil {
			t.Fatalf("DefaultNamespaces = %#v, want nil", options.Cache.DefaultNamespaces)
		}
	})

	t.Run("one namespace when configured", func(t *testing.T) {
		options := managerOptions(":8080", ":8081", "applications", true)

		if len(options.Cache.DefaultNamespaces) != 1 {
			t.Fatalf("DefaultNamespaces = %#v, want one namespace", options.Cache.DefaultNamespaces)
		}
		if _, ok := options.Cache.DefaultNamespaces["applications"]; !ok {
			t.Fatalf("DefaultNamespaces = %#v, want applications", options.Cache.DefaultNamespaces)
		}
	})
}
