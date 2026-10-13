mock_provider "cloudflare" {}

variables {
  account_id         = "00000000000000000000000000000000"
  bucket_name        = "example-packages"
  zone_id            = "11111111111111111111111111111111"
  public_hostname    = "packages.example.com"
  enable_cache_rules = true
}

run "respect_publisher_cache_control" {
  command = plan

  assert {
    condition = (
      cloudflare_ruleset.package_cdn_cache[0].rules[0].action_parameters.edge_ttl.mode == "respect_origin" &&
      cloudflare_ruleset.package_cdn_cache[0].rules[0].action_parameters.edge_ttl.default == null &&
      cloudflare_ruleset.package_cdn_cache[0].rules[0].action_parameters.browser_ttl.mode == "respect_origin" &&
      cloudflare_ruleset.package_cdn_cache[0].rules[0].action_parameters.browser_ttl.default == null
    )
    error_message = "Default cache rules must honor mutable metadata cache-control without a forced TTL."
  }
}

run "explicit_immutable_ttl_override" {
  command = plan

  variables {
    cache_edge_ttl_mode       = "override_origin"
    cache_browser_ttl_mode    = "override_origin"
    cache_edge_ttl_seconds    = 7200
    cache_browser_ttl_seconds = 600
  }

  assert {
    condition = (
      cloudflare_ruleset.package_cdn_cache[0].rules[0].action_parameters.edge_ttl.default == 7200 &&
      cloudflare_ruleset.package_cdn_cache[0].rules[0].action_parameters.browser_ttl.default == 600
    )
    error_message = "Explicit TTL overrides must remain configurable for immutable objects."
  }
}
