#ifndef DOCKPIPE_CONSOLE_LOCK_H
#define DOCKPIPE_CONSOLE_LOCK_H

#include <CoreFoundation/CoreFoundation.h>
#include <stdint.h>
#include <stdbool.h>

static Boolean DockpipeDictionaryBoolean(CFDictionaryRef dictionary, CFStringRef key, Boolean *value)
{
    CFTypeRef entry = CFDictionaryGetValue(dictionary, key);
    if (entry == NULL || CFGetTypeID(entry) != CFBooleanGetTypeID())
        return false;
    *value = CFBooleanGetValue((CFBooleanRef)entry);
    return true;
}

static Boolean DockpipeDictionaryUID(CFDictionaryRef dictionary, uint32_t *value)
{
    CFTypeRef entry = CFDictionaryGetValue(dictionary, CFSTR("kCGSSessionUserIDKey"));
    int64_t number = -1;
    if (entry == NULL || CFGetTypeID(entry) != CFNumberGetTypeID()
        || !CFNumberGetValue((CFNumberRef)entry, kCFNumberSInt64Type, &number)
        || number < 0 || number > UINT32_MAX)
        return false;
    *value = (uint32_t)number;
    return true;
}

// Apple publishes these registry properties and their exact container types in
// XNU's IOKitKeysPrivate.h. We read them through public IOKit APIs and validate
// every type at runtime; their availability is not assumed across OS versions.
// https://github.com/apple-oss-distributions/xnu/blob/main/iokit/IOKit/IOKitKeysPrivate.h
static Boolean DockpipeConsoleLockState(CFDictionaryRef properties, uint32_t currentUID, Boolean *locked)
{
    Boolean rootLocked = false;
    Boolean rootKnown = DockpipeDictionaryBoolean(properties, CFSTR("IOConsoleLocked"), &rootLocked);
    if (rootKnown && rootLocked) {
        *locked = true;
        return true;
    }

    CFTypeRef users = CFDictionaryGetValue(properties, CFSTR("IOConsoleUsers"));
    if (users == NULL || CFGetTypeID(users) != CFArrayGetTypeID())
        return false;
    CFArrayRef sessions = (CFArrayRef)users;
    CFIndex count = CFArrayGetCount(sessions);
    if (count < 1 || count > 64)
        return false;

    unsigned int activeSessions = 0;
    unsigned int ownerSessions = 0;
    Boolean ambiguous = false;
    Boolean ownerLocked = false;
    Boolean ownerLockKnown = false;
    for (CFIndex index = 0; index < count; index++) {
        CFTypeRef entry = CFArrayGetValueAtIndex(sessions, index);
        if (entry == NULL || CFGetTypeID(entry) != CFDictionaryGetTypeID()) {
            ambiguous = true;
            continue;
        }
        CFDictionaryRef session = (CFDictionaryRef)entry;
        Boolean onConsole = false;
        if (!DockpipeDictionaryBoolean(session, CFSTR("kCGSSessionOnConsoleKey"), &onConsole)) {
            ambiguous = true;
            continue;
        }
        if (!onConsole)
            continue;
        activeSessions++;

        Boolean loginDone = false;
        uint32_t owner = 0;
        if (!DockpipeDictionaryBoolean(session, CFSTR("kCGSessionLoginDoneKey"), &loginDone)
            || !DockpipeDictionaryUID(session, &owner)) {
            ambiguous = true;
            continue;
        }
        if (!loginDone || owner != currentUID)
            continue;
        ownerSessions++;

        Boolean sessionLocked = false;
        if (DockpipeDictionaryBoolean(session, CFSTR("CGSSessionScreenIsLocked"), &sessionLocked)) {
            // Positive lock evidence wins over a root-level false during OS
            // screen-saver/password-grace transitions.
            if (sessionLocked) {
                *locked = true;
                return true;
            }
            ownerLockKnown = true;
            ownerLocked = false;
        }
    }

    // A root-level false belongs to this operator only when one unambiguous,
    // logged-in console session is owned by the current OS account.
    if (ambiguous || activeSessions != 1 || ownerSessions != 1)
        return false;
    if (!ownerLockKnown && !rootKnown)
        return false;
    *locked = ownerLockKnown ? ownerLocked : rootLocked;
    return true;
}

#endif
