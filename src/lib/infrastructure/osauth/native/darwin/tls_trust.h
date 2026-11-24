#ifndef DOCKPIPE_TLS_TRUST_H
#define DOCKPIPE_TLS_TRUST_H

#import <Foundation/Foundation.h>
#import <Security/Security.h>

#include "tls_trust_policy.h"

#include <errno.h>
#include <stdint.h>
#include <stdio.h>

// One command per process, on its main thread. Only fixed stage codes and
// integer counters are retained; no certificate contents or labels are logged.
typedef struct {
    const char *stage;
    int64_t status;
    long domain;
    NSUInteger certificateCount;
    NSUInteger certificateBytes;
    CFIndex ruleCount;
} DockpipeTLSDiagnostic;

static DockpipeTLSDiagnostic DockpipeTLSDiagnostics;

static BOOL DockpipeTLSFailure(const char *stage, int64_t status)
{
    DockpipeTLSDiagnostics.stage = stage;
    DockpipeTLSDiagnostics.status = status;
    return NO;
}

static DockpipeTLSDisposition DockpipeTLSSettingsFailure(const char *stage)
{
    DockpipeTLSFailure(stage, 0);
    return DockpipeTLSUnsupported;
}

static int DockpipeTLSReportFailure(void)
{
    fprintf(stderr, "dockpipe-tls-trust unavailable stage=%s status=%lld domain=%ld certificates=%lu der_bytes=%lu rules=%ld\n",
        DockpipeTLSDiagnostics.stage == NULL ? "unknown" : DockpipeTLSDiagnostics.stage,
        (long long)DockpipeTLSDiagnostics.status, DockpipeTLSDiagnostics.domain,
        (unsigned long)DockpipeTLSDiagnostics.certificateCount,
        (unsigned long)DockpipeTLSDiagnostics.certificateBytes,
        (long)DockpipeTLSDiagnostics.ruleCount);
    return 2;
}

// Only public trust settings and certificate bytes are read. No identity query,
// keychain export, labels, private keys, trust mutation or network fetch is used.
// Apple documents empty settings arrays and omitted results as TrustRoot:
// https://developer.apple.com/documentation/security/sectrustsettingscopytrustsettings(_:_:_:)
static DockpipeTLSDisposition DockpipeTLSSettings(CFArrayRef settings)
{
    if (settings == NULL || CFGetTypeID(settings) != CFArrayGetTypeID())
        return DockpipeTLSSettingsFailure("settings.array-type");
    CFIndex count = CFArrayGetCount(settings);
    DockpipeTLSDiagnostics.ruleCount = count;
    if (count < 0 || count > 64)
        return DockpipeTLSSettingsFailure("settings.rule-limit");
    DockpipeTLSRule rules[64] = {0};
    // The API copies this row-level policy identifier separately from
    // kSecTrustSettingsPolicyString and SecPolicyCopyProperties.kSecPolicyName
    // (which are hostname constraints). Only recognize the known SSL-server
    // identifier paired with an actual unconstrained server-SSL policy.
    NSString *const policyNameKey = @"kSecTrustSettingsPolicyName";
    NSSet *allowedKeys = [NSSet setWithObjects:(__bridge id)kSecTrustSettingsPolicy,
        (__bridge id)kSecTrustSettingsResult, policyNameKey, nil];
    for (CFIndex index = 0; index < count; index++) {
        CFTypeRef entry = CFArrayGetValueAtIndex(settings, index);
        if (entry == NULL || CFGetTypeID(entry) != CFDictionaryGetTypeID())
            return DockpipeTLSSettingsFailure("settings.row-type");
        NSDictionary *row = (__bridge NSDictionary *)entry;
        DockpipeTLSRule rule = {.appliesToSSL = true};
        BOOL policyPropertiesRestricted = NO;
        id policy = row[(__bridge id)kSecTrustSettingsPolicy];
        if (policy != nil) {
            if (CFGetTypeID((__bridge CFTypeRef)policy) != SecPolicyGetTypeID())
                return DockpipeTLSSettingsFailure("settings.policy-type");
            NSDictionary *properties = CFBridgingRelease(SecPolicyCopyProperties((__bridge SecPolicyRef)policy));
            id oid = properties[(__bridge id)kSecPolicyOid];
            if (![oid isKindOfClass:[NSString class]])
                return DockpipeTLSSettingsFailure("settings.policy-properties");
            rule.appliesToSSL = [oid isEqual:(__bridge id)kSecPolicyAppleSSL];
            if (rule.appliesToSSL) {
                // kSecPolicyName is the TLS hostname, not a display label.
                NSSet *allowedProperties = [NSSet setWithObjects:(__bridge id)kSecPolicyOid,
                    (__bridge id)kSecPolicyClient, nil];
                for (id key in properties) {
                    if (![allowedProperties containsObject:key]) {
                        rule.restricted = true;
                        policyPropertiesRestricted = YES;
                    }
                }
                id client = properties[(__bridge id)kSecPolicyClient];
                if (client != nil) {
                    if (CFGetTypeID((__bridge CFTypeRef)client) != CFBooleanGetTypeID())
                        return DockpipeTLSSettingsFailure("settings.client-type");
                    // A client-only policy is not server TLS trust.
                    if ([client boolValue])
                        rule.appliesToSSL = false;
                }
            }
        }
        for (id key in row) {
            if (![allowedKeys containsObject:key])
                rule.restricted = true;
        }
        id policyName = row[policyNameKey];
        if (rule.appliesToSSL && policyName != nil
            && (policy == nil || ![policyName isKindOfClass:[NSString class]]
                || ![policyName isEqualToString:@"sslServer"]))
            return DockpipeTLSSettingsFailure("settings.policy-name-metadata");
        id result = row[(__bridge id)kSecTrustSettingsResult];
        rule.resultPresent = result != nil;
        if (rule.resultPresent) {
            if (CFGetTypeID((__bridge CFTypeRef)result) != CFNumberGetTypeID()
                || !CFNumberGetValue((__bridge CFNumberRef)result, kCFNumberIntType, &rule.result))
                return DockpipeTLSSettingsFailure("settings.result-type");
        }
        if (rule.appliesToSSL && rule.restricted) {
            const char *reason = "settings.unknown-constraint";
            if (row[(__bridge id)kSecTrustSettingsApplication] != nil)
                reason = "settings.application-constraint";
            else if (row[(__bridge id)kSecTrustSettingsPolicyString] != nil)
                reason = "settings.hostname-constraint";
            else if (row[(__bridge id)kSecTrustSettingsAllowedError] != nil)
                reason = "settings.allowed-error";
            else if (row[(__bridge id)kSecTrustSettingsKeyUsage] != nil)
                reason = "settings.key-usage";
            else if (policyPropertiesRestricted)
                reason = "settings.policy-property-constraint";
            return DockpipeTLSSettingsFailure(reason);
        }
        if (rule.appliesToSSL && rule.resultPresent && rule.result == 3)
            return DockpipeTLSSettingsFailure("settings.deny");
        rules[index] = rule;
    }
    const DockpipeTLSDisposition disposition = DockpipeClassifyTLSRules(rules, (size_t)count);
    if (disposition == DockpipeTLSUnsupported)
        return DockpipeTLSSettingsFailure("settings.result-unsupported");
    return disposition;
}

static BOOL DockpipeTLSReadSettings(SecCertificateRef certificate, SecTrustSettingsDomain domain,
    DockpipeTLSDisposition *disposition, BOOL *present)
{
    DockpipeTLSDiagnostics.domain = (long)domain;
    DockpipeTLSDiagnostics.ruleCount = 0;
    CFArrayRef settings = NULL;
    OSStatus status = SecTrustSettingsCopyTrustSettings(certificate, domain, &settings);
    *present = status == errSecSuccess;
    *disposition = DockpipeTLSNoDecision;
    if (status == errSecItemNotFound || status == errSecNoTrustSettings) {
        if (settings != NULL)
            CFRelease(settings);
        return YES;
    }
    if (status != errSecSuccess) {
        if (settings != NULL)
            CFRelease(settings);
        return DockpipeTLSFailure("read-settings", status);
    }
    *disposition = DockpipeTLSSettings(settings);
    if (settings != NULL)
        CFRelease(settings);
    return *disposition != DockpipeTLSUnsupported;
}

static BOOL DockpipeTLSAddCertificates(CFArrayRef certificates, NSMutableDictionary *candidates,
    NSUInteger *totalBytes)
{
    if (certificates == NULL || CFGetTypeID(certificates) != CFArrayGetTypeID()
        || CFArrayGetCount(certificates) > 512)
        return DockpipeTLSFailure("certificates.array-or-limit", 0);
    for (id value in (__bridge NSArray *)certificates) {
        if (CFGetTypeID((__bridge CFTypeRef)value) != SecCertificateGetTypeID())
            return DockpipeTLSFailure("certificates.entry-type", 0);
        NSData *der = CFBridgingRelease(SecCertificateCopyData((__bridge SecCertificateRef)value));
        if (der == nil || der.length == 0 || der.length > 16 * 1024)
            return DockpipeTLSFailure("certificates.der-size", 0);
        if (candidates[der] == nil) {
            if (candidates.count >= 512 || *totalBytes + der.length > 2 * 1024 * 1024)
                return DockpipeTLSFailure("certificates.aggregate-limit", 0);
            candidates[der] = value;
            *totalBytes += der.length;
            DockpipeTLSDiagnostics.certificateCount = candidates.count;
            DockpipeTLSDiagnostics.certificateBytes = *totalBytes;
        }
    }
    return YES;
}

static BOOL DockpipeTLSCurrentlyTrusted(SecCertificateRef certificate)
{
    DockpipeTLSDiagnostics.domain = -1;
    DockpipeTLSDiagnostics.ruleCount = 0;
    SecPolicyRef policy = SecPolicyCreateSSL(true, NULL);
    if (policy == NULL)
        return DockpipeTLSFailure("ssl-policy-create", 0);
    SecTrustRef trust = NULL;
    OSStatus status = SecTrustCreateWithCertificates(certificate, policy, &trust);
    CFRelease(policy);
    if (status != errSecSuccess || trust == NULL) {
        if (trust != NULL)
            CFRelease(trust);
        return DockpipeTLSFailure("ssl-trust-create", status);
    }
    // Positive evaluation checks effective account/domain state for explicit
    // additions, including whether user trust is disabled. No AIA/OCSP fetches.
    status = SecTrustSetNetworkFetchAllowed(trust, false);
    if (status != errSecSuccess) {
        CFRelease(trust);
        return DockpipeTLSFailure("ssl-disable-network", status);
    }
    CFErrorRef error = NULL;
    BOOL trusted = SecTrustEvaluateWithError(trust, &error);
    if (!trusted)
        DockpipeTLSFailure("ssl-evaluation", error == NULL ? 0 : (int64_t)CFErrorGetCode(error));
    if (error != NULL)
        CFRelease(error);
    CFRelease(trust);
    return trusted;
}

static int DockpipeTLSExportTrust(void)
{
    DockpipeTLSDiagnostics = (DockpipeTLSDiagnostic){.stage = "collector", .domain = -1};
    const SecTrustSettingsDomain domains[] = {kSecTrustSettingsDomainUser,
        kSecTrustSettingsDomainAdmin, kSecTrustSettingsDomainSystem};
    NSMutableDictionary *candidates = [NSMutableDictionary dictionary];
    NSUInteger totalBytes = 0;
    // Explicit domain-wide defaults cannot be represented by a finite set of
    // extra roots (nor can deny defaults subtract the consumer's existing roots).
    for (size_t index = 0; index < sizeof(domains) / sizeof(domains[0]); index++) {
        DockpipeTLSDisposition disposition;
        BOOL present = NO;
        if (!DockpipeTLSReadSettings(kSecTrustSettingsDefaultRootCertSetting, domains[index],
                &disposition, &present))
            return DockpipeTLSReportFailure();
        if (present) {
            DockpipeTLSFailure("default-rule-present", 0);
            return DockpipeTLSReportFailure();
        }
    }
    OSStatus status;
    BOOL valid;
    // Export only explicit user/admin additional SSL anchors. Default platform
    // roots carry trust-asset constraints that a DER-only snapshot cannot encode.
    // Chromium retains its normal public roots; system settings are inspected
    // below for restrictions on the selected explicit candidates.
    for (size_t index = 0; index < 2; index++) {
        DockpipeTLSDiagnostics.domain = (long)domains[index];
        CFArrayRef certificates = NULL;
        status = SecTrustSettingsCopyCertificates(domains[index], &certificates);
        if (status == errSecNoTrustSettings) {
            if (certificates != NULL)
                CFRelease(certificates);
            continue;
        }
        if (status != errSecSuccess)
            DockpipeTLSFailure("enumerate-certificates", status);
        valid = status == errSecSuccess
            && DockpipeTLSAddCertificates(certificates, candidates, &totalBytes);
        if (certificates != NULL)
            CFRelease(certificates);
        if (!valid)
            return DockpipeTLSReportFailure();
    }
    NSMutableArray *encoded = [NSMutableArray array];
    for (NSData *der in candidates) {
        SecCertificateRef certificate = (__bridge SecCertificateRef)candidates[der];
        BOOL selected = NO;
        BOOL decided = NO;
        for (size_t index = 0; index < sizeof(domains) / sizeof(domains[0]); index++) {
            DockpipeTLSDisposition disposition;
            BOOL present = NO;
            if (!DockpipeTLSReadSettings(certificate, domains[index], &disposition, &present))
                return DockpipeTLSReportFailure();
            // Highest applicable domain wins. Still inspect lower domains and
            // refuse unsupported restrictions rather than flattening them.
            if (!decided && index < 2 && disposition == DockpipeTLSAnchor) {
                selected = YES;
                decided = YES;
            }
        }
        if (!selected)
            continue;
        if (!DockpipeTLSCurrentlyTrusted(certificate))
            return DockpipeTLSReportFailure();
        [encoded addObject:[der base64EncodedStringWithOptions:0]];
    }
    [encoded sortUsingSelector:@selector(compare:)];
    NSDictionary *value = @{@"schema": @"dockpipe.remote.network-trust-material/v1",
        @"certificates": encoded};
    NSData *data = [NSJSONSerialization dataWithJSONObject:value options:NSJSONWritingSortedKeys error:nil];
    if (data == nil || data.length > 3 * 1024 * 1024) {
        DockpipeTLSFailure("serialize-json", 0);
        return DockpipeTLSReportFailure();
    }
    if (fwrite(data.bytes, 1, data.length, stdout) != data.length) {
        DockpipeTLSFailure("write-json", errno);
        return DockpipeTLSReportFailure();
    }
    return 0;
}

#endif
