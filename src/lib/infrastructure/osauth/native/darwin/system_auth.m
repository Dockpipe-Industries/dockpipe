#import <Foundation/Foundation.h>
#import <LocalAuthentication/LocalAuthentication.h>
#import <CoreGraphics/CoreGraphics.h>
#import <IOKit/IOKitLib.h>
#import <mach/mach_time.h>
#import <math.h>
#import <pwd.h>
#import <stdint.h>
#import <stdio.h>
#import <string.h>
#import <unistd.h>

#include "console_lock.h"

static int authenticate(void)
{
    LAContext *context = [[LAContext alloc] init];
    context.touchIDAuthenticationAllowableReuseDuration = 0;
    context.localizedCancelTitle = @"Cancel";
    NSError *error = nil;
    if (![context canEvaluatePolicy:LAPolicyDeviceOwnerAuthentication error:&error])
        return 2;

    __block BOOL completed = NO;
    __block BOOL authenticated = NO;
    [context evaluatePolicy:LAPolicyDeviceOwnerAuthentication
            localizedReason:@"Unlock Dockpipe remote access for this computer."
                      reply:^(BOOL success, NSError *replyError) {
        (void)replyError;
        dispatch_async(dispatch_get_main_queue(), ^{
            authenticated = success;
            completed = YES;
        });
    }];

    NSTimeInterval deadline = NSProcessInfo.processInfo.systemUptime + 110;
    while (!completed && NSProcessInfo.processInfo.systemUptime < deadline)
        [[NSRunLoop mainRunLoop] runUntilDate:[NSDate dateWithTimeIntervalSinceNow:0.1]];

    [context invalidate];
    return completed && authenticated ? 0 : 1;
}

static BOOL registryConsoleLock(BOOL *locked)
{
    io_registry_entry_t root = IORegistryGetRootEntry(kIOMainPortDefault);
    if (root == IO_OBJECT_NULL)
        return NO;
    CFMutableDictionaryRef properties = NULL;
    kern_return_t result = IORegistryEntryCreateCFProperties(root, &properties, kCFAllocatorDefault, 0);
    IOObjectRelease(root);
    if (result != KERN_SUCCESS || properties == NULL)
        return NO;
    Boolean value = true;
    Boolean known = DockpipeConsoleLockState(properties, getuid(), &value);
    CFRelease(properties);
    if (known)
        *locked = value;
    return known;
}

static int snapshot(void)
{
    CFTimeInterval idle = CGEventSourceSecondsSinceLastEventType(
        kCGEventSourceStateHIDSystemState, kCGAnyInputEventType);
    BOOL idleKnown = isfinite(idle) && idle >= 0 && idle <= 365 * 24 * 60 * 60;
    BOOL locked = YES;
    BOOL lockKnown = NO;
    CFDictionaryRef session = CGSessionCopyCurrentDictionary();
    if (session != NULL) {
        Boolean onConsole = false;
        Boolean loginDone = false;
        BOOL consoleKnown = DockpipeDictionaryBoolean(session, kCGSessionOnConsoleKey, &onConsole);
        BOOL loginKnown = DockpipeDictionaryBoolean(session, kCGSessionLoginDoneKey, &loginDone);
        CFNumberRef userID = CFDictionaryGetValue(session, kCGSessionUserIDKey);
        uid_t owner = (uid_t)-1;
        BOOL ownerKnown = userID != NULL && CFGetTypeID(userID) == CFNumberGetTypeID()
            && CFNumberGetValue(userID, kCFNumberIntType, &owner);
        if (consoleKnown && loginKnown && ownerKnown) {
            if (!onConsole || !loginDone || owner != getuid()) {
                locked = YES;
                lockKnown = YES;
            } else {
                // This OS-supplied dictionary entry is not guaranteed on every
                // macOS release. Missing or differently typed means unknown.
                Boolean sessionLocked = true;
                lockKnown = DockpipeDictionaryBoolean(session, CFSTR("CGSSessionScreenIsLocked"), &sessionLocked);
                if (lockKnown)
                    locked = sessionLocked;
            }
        }
        CFRelease(session);
    }
    BOOL registryLocked = YES;
    BOOL registryKnown = registryConsoleLock(&registryLocked);
    if (registryKnown && (registryLocked || !lockKnown)) {
        locked = registryLocked;
        lockKnown = YES;
    }

    mach_timebase_info_data_t timebase;
    BOOL sleepKnown = mach_timebase_info(&timebase) == KERN_SUCCESS && timebase.denom != 0;
    uint64_t suspendedNanos = 0;
    if (sleepKnown) {
        uint64_t continuous = mach_continuous_time();
        uint64_t absolute = mach_absolute_time();
        uint64_t suspended = continuous > absolute ? continuous - absolute : 0;
        __uint128_t nanos = (__uint128_t)suspended * timebase.numer / timebase.denom;
        sleepKnown = nanos <= UINT64_MAX;
        if (sleepKnown)
            suspendedNanos = (uint64_t)nanos;
    }

    NSDictionary *value = @{
        @"schema": @"dockpipe.osauth/v1",
        @"idle_seconds": @(idleKnown ? idle : 0),
        @"idle_known": @(idleKnown),
        @"locked": @(locked),
        @"lock_known": @(lockKnown),
        @"sleep_known": @(sleepKnown),
        @"suspended_nanoseconds": @(suspendedNanos)
    };
    NSData *data = [NSJSONSerialization dataWithJSONObject:value options:0 error:nil];
    if (data == nil || data.length > 4096)
        return 2;
    return fwrite(data.bytes, 1, data.length, stdout) == data.length ? 0 : 2;
}

static int securityDirectory(void)
{
    // Native account lookup includes Directory Services accounts when the Go
    // CLI is built without cgo. HOME and other process environment is ignored.
    struct passwd *account = getpwuid(getuid());
    if (account == NULL || account->pw_dir == NULL)
        return 2;
    NSString *home = [NSString stringWithUTF8String:account->pw_dir];
    if (home == nil || !home.isAbsolutePath || [home isEqualToString:@"/"])
        return 2;
    NSDictionary *value = @{@"schema": @"dockpipe.osauth/v1", @"home": home};
    NSData *data = [NSJSONSerialization dataWithJSONObject:value options:0 error:nil];
    if (data == nil || data.length > 4096)
        return 2;
    return fwrite(data.bytes, 1, data.length, stdout) == data.length ? 0 : 2;
}

int main(int argc, const char *argv[])
{
    @autoreleasepool {
        if (argc != 2)
            return 2;
        if (strcmp(argv[1], "--authenticate") == 0)
            return authenticate();
        if (strcmp(argv[1], "--snapshot") == 0)
            return snapshot();
        if (strcmp(argv[1], "--security-directory") == 0)
            return securityDirectory();
        return 2;
    }
}
