#import <Foundation/Foundation.h>

#include "console_lock.h"

static NSDictionary *ownerSession(id lockValue)
{
    NSMutableDictionary *session = [@{
        @"kCGSSessionUserIDKey": @501,
        @"kCGSSessionOnConsoleKey": @YES,
        @"kCGSessionLoginDoneKey": @YES
    } mutableCopy];
    if (lockValue != nil)
        session[@"CGSSessionScreenIsLocked"] = lockValue;
    return session;
}

static void expectState(NSDictionary *properties, BOOL expectedKnown, BOOL expectedLocked)
{
    Boolean locked = true;
    Boolean known = DockpipeConsoleLockState((__bridge CFDictionaryRef)properties, 501, &locked);
    if (known != expectedKnown || (known && locked != expectedLocked))
        abort();
}

int main(void)
{
    @autoreleasepool {
        expectState(@{}, NO, YES);
        expectState(@{@"IOConsoleLocked": @NO}, NO, YES);
        expectState(@{@"IOConsoleLocked": @NO, @"IOConsoleUsers": @[]}, NO, YES);
        expectState(@{@"IOConsoleLocked": @NO, @"IOConsoleUsers": @"invalid"}, NO, YES);
        expectState(@{@"IOConsoleLocked": @YES}, YES, YES);
        expectState(@{@"IOConsoleUsers": @[ownerSession(nil)]}, NO, YES);
        expectState(@{@"IOConsoleUsers": @[ownerSession(@NO)]}, YES, NO);
        expectState(@{@"IOConsoleUsers": @[ownerSession(@YES)]}, YES, YES);
        expectState(@{@"IOConsoleLocked": @NO, @"IOConsoleUsers": @[ownerSession(nil)]}, YES, NO);
        expectState(@{@"IOConsoleLocked": @NO, @"IOConsoleUsers": @[ownerSession(@YES)]}, YES, YES);
        expectState(@{@"IOConsoleLocked": @YES, @"IOConsoleUsers": @[ownerSession(@NO)]}, YES, YES);
        expectState(@{@"IOConsoleLocked": @0, @"IOConsoleUsers": @[ownerSession(nil)]}, NO, YES);
        expectState(@{@"IOConsoleUsers": @[ownerSession(@0)]}, NO, YES);

        NSMutableDictionary *otherOwner = [ownerSession(@NO) mutableCopy];
        otherOwner[@"kCGSSessionUserIDKey"] = @502;
        expectState(@{@"IOConsoleLocked": @NO, @"IOConsoleUsers": @[otherOwner]}, NO, YES);
        expectState(@{@"IOConsoleLocked": @NO, @"IOConsoleUsers": @[ownerSession(nil), otherOwner]}, NO, YES);
        expectState(@{@"IOConsoleLocked": @NO, @"IOConsoleUsers": @[ownerSession(nil), ownerSession(nil)]}, NO, YES);

        NSMutableDictionary *inactive = [ownerSession(@NO) mutableCopy];
        inactive[@"kCGSSessionOnConsoleKey"] = @NO;
        expectState(@{@"IOConsoleLocked": @NO, @"IOConsoleUsers": @[inactive]}, NO, YES);
        NSMutableDictionary *loginPending = [ownerSession(@NO) mutableCopy];
        loginPending[@"kCGSessionLoginDoneKey"] = @NO;
        expectState(@{@"IOConsoleLocked": @NO, @"IOConsoleUsers": @[loginPending]}, NO, YES);
        NSMutableDictionary *invalidOwner = [ownerSession(@NO) mutableCopy];
        invalidOwner[@"kCGSSessionUserIDKey"] = @YES;
        expectState(@{@"IOConsoleLocked": @NO, @"IOConsoleUsers": @[invalidOwner]}, NO, YES);
        expectState(@{@"IOConsoleLocked": @NO, @"IOConsoleUsers": @[ownerSession(nil), @"invalid"]}, NO, YES);
        return 0;
    }
}
