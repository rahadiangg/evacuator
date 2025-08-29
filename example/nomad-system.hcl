job "evacuator" {
    datacenters = ["dc1"]
  type        = "system"

  group "evacuator" {
    task "evacuator" {
      driver = "docker"

      identity {
        change_mode = "restart"
        env         = true
      }

      config {
        image      = "rahadiangg/evacuator-nightly:test"
        force_pull = true
      }

      env {
        HANDLER_NOMAD_ENABLED         = "true"
        LOG_LEVEL                     = "debug"
        NODE_NAME                     = "${attr.unique.hostname}"
        # PROVIDER_NAME                 = "dummy"
        # PROVIDER_DUMMY_ENABLED        = "true"
        # PROVIDER_DUMMY_DETECTION_WAIT = "20s"

        # Nomad address (used for API communication)
        NOMAD_ADDR                    = "${attr.unique.network.ip-address}"
      }

      resources {
        cpu        = 100
        memory     = 128
        memory_max = 128
      }
    }
  }
}