#import <Foundation/Foundation.h>

#include "tls_trust.h"

#include <string.h>

static void expectSettings(NSArray *settings, DockpipeTLSDisposition expected)
{
    if (DockpipeTLSSettings((__bridge CFArrayRef)settings) != expected)
        abort();
}

static void expectFailureReason(NSArray *settings, const char *reason)
{
    expectSettings(settings, DockpipeTLSUnsupported);
    if (DockpipeTLSDiagnostics.stage == NULL || strcmp(DockpipeTLSDiagnostics.stage, reason) != 0)
        abort();
}

int main(void)
{
    @autoreleasepool {
        // This fixture never invokes the collector, reads a trust store, changes
        // trust, prompts for authentication or opens a network connection.
        (void)DockpipeTLSExportTrust;
        expectSettings(@[], DockpipeTLSAnchor);
        expectSettings(@[@{}], DockpipeTLSAnchor);
        expectSettings(@[@{(__bridge id)kSecTrustSettingsResult: @1}], DockpipeTLSAnchor);
        expectSettings(@[@{(__bridge id)kSecTrustSettingsResult: @2}], DockpipeTLSAnchor);
        expectSettings(@[@{(__bridge id)kSecTrustSettingsResult: @3}], DockpipeTLSUnsupported);
        expectSettings(@[@{(__bridge id)kSecTrustSettingsResult: @4}], DockpipeTLSNoDecision);
        expectSettings(@[@{(__bridge id)kSecTrustSettingsResult: @YES}], DockpipeTLSUnsupported);
        expectSettings(@[@{(__bridge id)kSecTrustSettingsResult: @"1"}], DockpipeTLSUnsupported);
        expectSettings(@[@{(__bridge id)kSecTrustSettingsPolicyString: @"localhost"}], DockpipeTLSUnsupported);
        expectSettings(@[@{(__bridge id)kSecTrustSettingsApplication: @"application"}], DockpipeTLSUnsupported);
        expectSettings(@[@{(__bridge id)kSecTrustSettingsAllowedError: @0}], DockpipeTLSUnsupported);
        expectSettings(@[@{(__bridge id)kSecTrustSettingsKeyUsage: @0}], DockpipeTLSUnsupported);
        expectSettings(@[@{@"unknown": @1}], DockpipeTLSUnsupported);
        expectSettings(@[@"invalid"], DockpipeTLSUnsupported);
        expectSettings(nil, DockpipeTLSUnsupported);
        NSMutableArray *oversized = [NSMutableArray array];
        for (int index = 0; index < 65; index++)
            [oversized addObject:@{}];
        expectSettings(oversized, DockpipeTLSUnsupported);

        SecPolicyRef ssl = SecPolicyCreateSSL(true, NULL);
        SecPolicyRef host = SecPolicyCreateSSL(true, CFSTR("localhost"));
        SecPolicyRef client = SecPolicyCreateSSL(false, NULL);
        SecPolicyRef other = SecPolicyCreateBasicX509();
        if (ssl == NULL || host == NULL || client == NULL || other == NULL)
            abort();
        expectSettings(@[@{(__bridge id)kSecTrustSettingsPolicy: (__bridge id)ssl}], DockpipeTLSAnchor);
        expectSettings(@[@{(__bridge id)kSecTrustSettingsPolicy: (__bridge id)ssl,
            @"kSecTrustSettingsPolicyName": @"sslServer"}], DockpipeTLSAnchor);
        expectFailureReason(@[@{(__bridge id)kSecTrustSettingsPolicy: (__bridge id)ssl,
            @"kSecTrustSettingsPolicyName": @"sslClient"}], "settings.policy-name-metadata");
        expectFailureReason(@[@{(__bridge id)kSecTrustSettingsPolicy: (__bridge id)ssl,
            @"kSecTrustSettingsPolicyName": @7}], "settings.policy-name-metadata");
        expectFailureReason(@[@{@"kSecTrustSettingsPolicyName": @"sslServer"}], "settings.policy-name-metadata");
        expectFailureReason(@[@{(__bridge id)kSecTrustSettingsPolicy: (__bridge id)host,
            @"kSecTrustSettingsPolicyName": @"sslServer"}], "settings.policy-property-constraint");
        expectFailureReason(@[@{(__bridge id)kSecTrustSettingsPolicy: (__bridge id)ssl,
            @"kSecTrustSettingsPolicyName": @"sslServer",
            (__bridge id)kSecTrustSettingsPolicyString: @"localhost"}], "settings.hostname-constraint");
        expectSettings(@[@{(__bridge id)kSecTrustSettingsPolicy: (__bridge id)host}], DockpipeTLSUnsupported);
        expectSettings(@[@{(__bridge id)kSecTrustSettingsPolicy: (__bridge id)client}], DockpipeTLSNoDecision);
        expectSettings(@[@{(__bridge id)kSecTrustSettingsPolicy: (__bridge id)other}], DockpipeTLSNoDecision);
        expectSettings(@[@{(__bridge id)kSecTrustSettingsPolicy: (__bridge id)other,
            (__bridge id)kSecTrustSettingsResult: @3}], DockpipeTLSNoDecision);
        expectSettings(@[@{}, @{(__bridge id)kSecTrustSettingsPolicyString: @"localhost"}], DockpipeTLSUnsupported);
        expectFailureReason(@[@{(__bridge id)kSecTrustSettingsResult: @3}], "settings.deny");
        expectFailureReason(@[@{(__bridge id)kSecTrustSettingsApplication: @"private-application-fixture"}],
            "settings.application-constraint");
        expectFailureReason(@[@{(__bridge id)kSecTrustSettingsPolicyString: @"private-host-fixture"}],
            "settings.hostname-constraint");
        expectFailureReason(@[@{(__bridge id)kSecTrustSettingsAllowedError: @0}], "settings.allowed-error");
        expectFailureReason(@[@{(__bridge id)kSecTrustSettingsKeyUsage: @0}], "settings.key-usage");
        expectFailureReason(@[@{(__bridge id)kSecTrustSettingsPolicy: (__bridge id)host}],
            "settings.policy-property-constraint");
        expectFailureReason(@[@{@"private-unknown-key-fixture": @1}], "settings.unknown-constraint");
        expectFailureReason(oversized, "settings.rule-limit");
        if (DockpipeTLSDiagnostics.ruleCount != 65)
            abort();
        DockpipeTLSFailure("read-settings", -50);
        if (DockpipeTLSDiagnostics.status != -50)
            abort();
        CFRelease(ssl);
        CFRelease(host);
        CFRelease(client);
        CFRelease(other);
        return 0;
    }
}
