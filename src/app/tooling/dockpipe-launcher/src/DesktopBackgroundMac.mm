#include "DesktopBackground.h"

#import <AppKit/AppKit.h>
#include <memory>

DesktopBackgroundResult resolveMacDesktopBackground()
{
    // AppKit's screen/workspace access stays on the main thread. The calling
    // worker has a finite wait; a late block retains only its own small result.
    const auto selectedPath = std::make_shared<QString>();
    const auto readPath = ^{
        @autoreleasepool {
            NSScreen *screen = NSScreen.mainScreen;
            NSURL *url = screen ? [NSWorkspace.sharedWorkspace desktopImageURLForScreen:screen] : nil;
            if (url.isFileURL)
                *selectedPath = QString::fromNSString(url.path);
        }
    };
    if (NSThread.isMainThread) {
        readPath();
    } else {
        dispatch_semaphore_t completed = dispatch_semaphore_create(0);
        dispatch_async(dispatch_get_main_queue(), ^{
            readPath();
            dispatch_semaphore_signal(completed);
        });
        if (dispatch_semaphore_wait(completed, dispatch_time(DISPATCH_TIME_NOW, 2 * NSEC_PER_SEC)) != 0)
            return {{}, QStringLiteral("macOS wallpaper lookup timed out. Choose an image instead.")};
    }
    if (selectedPath->isEmpty())
        return {{}, QStringLiteral("macOS did not provide a local wallpaper image for its main screen. Dynamic or video wallpapers may not expose a static image.")};
    return DesktopBackground::localImage(*selectedPath);
}
