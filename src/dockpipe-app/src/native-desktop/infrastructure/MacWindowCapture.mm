#include "MacWindowCapture.h"

#include <QCoreApplication>
#include <QPointer>
#include <QTimer>
#include <atomic>
#include <mutex>

#import <AppKit/AppKit.h>
#import <ScreenCaptureKit/ScreenCaptureKit.h>
#import <CoreMedia/CoreMedia.h>
#import <CoreVideo/CoreVideo.h>

#if __MAC_OS_X_VERSION_MAX_ALLOWED >= 140000
namespace {
struct FrameDelivery {
    std::atomic_bool active{true};
    std::atomic_bool errorPosted{false};
    std::atomic_bool visible{true};
    std::mutex mutex;
    QImage latest;
    bool queued = false;
    // Read and clear only on the Qt/main thread.
    QPointer<MacWindowCapture> owner;
};

void deliverError(const std::shared_ptr<FrameDelivery> &delivery, const QString &message)
{
    if (!delivery->active || delivery->errorPosted.exchange(true))
        return;
    QMetaObject::invokeMethod(qApp, [delivery, message] {
        if (delivery->active && delivery->owner)
            emit delivery->owner->failed(message);
    }, Qt::QueuedConnection);
}
}

API_AVAILABLE(macos(14.0))
@interface DockpipeWindowCaptureBridge : NSObject <SCContentSharingPickerObserver, SCStreamOutput, SCStreamDelegate> {
@public
    std::shared_ptr<FrameDelivery> delivery;
    SCStream *stream;
    dispatch_queue_t framesQueue;
    BOOL waiting;
    BOOL configurationPending;
    BOOL appliedVisibility;
}
- (void)updateVisibilityConfiguration;
@end

// The system picker is process-global. Limit selection to one active dashboard
// share so callbacks cannot accidentally grant another panel's pending request.
static __weak DockpipeWindowCaptureBridge *activeBridge;

API_AVAILABLE(macos(14.0))
static SCStreamConfiguration *windowConfiguration(bool visible)
{
    SCStreamConfiguration *configuration = [[SCStreamConfiguration alloc] init];
    configuration.width = 1280;
    configuration.height = 720;
    configuration.minimumFrameInterval = CMTimeMake(1, visible ? 15 : 1);
    configuration.queueDepth = 3;
    configuration.pixelFormat = kCVPixelFormatType_32BGRA;
    configuration.capturesAudio = NO;
    configuration.showsCursor = YES;
    configuration.scalesToFit = YES;
    return configuration;
}

@implementation DockpipeWindowCaptureBridge
- (void)updateVisibilityConfiguration
{
    if (!delivery->active || !stream || configurationPending || appliedVisibility == delivery->visible)
        return;
    const bool requestedVisibility = delivery->visible;
    configurationPending = YES;
    [stream updateConfiguration:windowConfiguration(requestedVisibility) completionHandler:^(NSError *error) {
        dispatch_async(dispatch_get_main_queue(), ^{
            self->configurationPending = NO;
            if (!self->delivery->active)
                return;
            if (error) {
                deliverError(self->delivery, QString::fromNSString(error.localizedDescription));
                return;
            }
            self->appliedVisibility = requestedVisibility;
            // One update in flight, with the newest visibility applied next.
            [self updateVisibilityConfiguration];
        });
    }];
}

- (void)contentSharingPicker:(SCContentSharingPicker *)picker didCancelForStream:(SCStream *)selectedStream
{
    dispatch_async(dispatch_get_main_queue(), ^{
        if (activeBridge != self || (selectedStream && selectedStream != self->stream))
            return;
        auto token = self->delivery;
        if (token->active && token->owner)
            emit token->owner->cancelled();
    });
}

- (void)contentSharingPickerStartDidFailWithError:(NSError *)error
{
    dispatch_async(dispatch_get_main_queue(), ^{
        if (activeBridge == self)
            deliverError(self->delivery, QString::fromNSString(error.localizedDescription));
    });
}

- (void)contentSharingPicker:(SCContentSharingPicker *)picker didUpdateWithFilter:(SCContentFilter *)filter forStream:(SCStream *)selectedStream
{
    // Picker callbacks are marshalled to main before touching Qt or stream state.
    dispatch_async(dispatch_get_main_queue(), ^{
        if (!self->delivery->active || activeBridge != self || !self->waiting)
            return;
        self->waiting = NO;
        if (self->delivery->owner)
            self->delivery->owner->setVisible(self->delivery->visible);
        SCStreamConfiguration *configuration = windowConfiguration(self->delivery->visible);
        self->appliedVisibility = self->delivery->visible;
        self->stream = [[SCStream alloc] initWithFilter:filter configuration:configuration delegate:self];
        NSError *outputError = nil;
        if (![self->stream addStreamOutput:self type:SCStreamOutputTypeScreen sampleHandlerQueue:self->framesQueue error:&outputError]) {
            deliverError(self->delivery, QString::fromNSString(outputError.localizedDescription));
            return;
        }
        auto token = self->delivery;
        SCStream *startingStream = self->stream;
        [startingStream startCaptureWithCompletionHandler:^(NSError *error) {
            if (!token->active) {
                // Stop racing an asynchronous start must also stop its late result.
                [startingStream stopCaptureWithCompletionHandler:nil];
                return;
            }
            if (error)
                deliverError(token, QString::fromNSString(error.localizedDescription));
        }];
    });
}

- (void)stream:(SCStream *)capture didStopWithError:(NSError *)error
{
    deliverError(delivery, QString::fromNSString(error.localizedDescription));
}

- (void)stream:(SCStream *)capture didOutputSampleBuffer:(CMSampleBufferRef)sampleBuffer ofType:(SCStreamOutputType)type
{
    auto token = delivery;
    if (!token->active || !token->visible || type != SCStreamOutputTypeScreen || !CMSampleBufferIsValid(sampleBuffer))
        return;
    CFArrayRef attachments = CMSampleBufferGetSampleAttachmentsArray(sampleBuffer, false);
    if (!attachments || CFArrayGetCount(attachments) == 0)
        return;
    NSDictionary *metadata = (__bridge NSDictionary *)CFArrayGetValueAtIndex(attachments, 0);
    NSNumber *status = metadata[SCStreamFrameInfoStatus];
    if (![status isKindOfClass:NSNumber.class] || status.integerValue != SCFrameStatusComplete)
        return;
    CVPixelBufferRef pixels = CMSampleBufferGetImageBuffer(sampleBuffer);
    if (!pixels || CVPixelBufferGetPixelFormatType(pixels) != kCVPixelFormatType_32BGRA)
        return;
    const size_t width = CVPixelBufferGetWidth(pixels);
    const size_t height = CVPixelBufferGetHeight(pixels);
    const size_t stride = CVPixelBufferGetBytesPerRow(pixels);
    if (width == 0 || height == 0 || width > 1280 || height > 720 || stride < width * 4 || stride > 8192) {
        deliverError(token, QStringLiteral("macOS returned a frame outside the 1280×720 capture limit."));
        return;
    }
    if (CVPixelBufferLockBaseAddress(pixels, kCVPixelBufferLock_ReadOnly) != kCVReturnSuccess)
        return;
    QImage image;
    const auto *bytes = static_cast<const uchar *>(CVPixelBufferGetBaseAddress(pixels));
    if (bytes)
        image = QImage(bytes, int(width), int(height), qsizetype(stride), QImage::Format_ARGB32_Premultiplied).copy();
    CVPixelBufferUnlockBaseAddress(pixels, kCVPixelBufferLock_ReadOnly);
    if (image.isNull())
        return;
    {
        std::lock_guard<std::mutex> lock(token->mutex);
        if (!token->active)
            return;
        token->latest = std::move(image);
        if (token->queued)
            return;
        token->queued = true;
    }
    QMetaObject::invokeMethod(qApp, [token] {
        QImage latest;
        {
            std::lock_guard<std::mutex> lock(token->mutex);
            latest = std::move(token->latest);
            token->queued = false;
        }
        if (token->active && token->visible && token->owner && !latest.isNull())
            emit token->owner->frameReady(latest);
    }, Qt::QueuedConnection);
}
@end
#endif

struct MacWindowCapture::State {
    QTimer deadline;
    int hiddenDeadlineRemaining = -1;
#if __MAC_OS_X_VERSION_MAX_ALLOWED >= 140000
    DockpipeWindowCaptureBridge *bridge = nil;
#endif
};

MacWindowCapture::MacWindowCapture(QObject *parent)
    : QObject(parent), m_state(std::make_unique<State>())
{
    m_state->deadline.setSingleShot(true);
    connect(&m_state->deadline, &QTimer::timeout, this, [this] {
        stop();
        emit failed(tr("Window sharing timed out. Choose a window again."));
    });
    connect(this, &MacWindowCapture::frameReady, this, [this] {
        if (m_state->deadline.isActive()) {
            m_state->deadline.stop();
            emit statusChanged(tr("Shared macOS window · View only. Use the original window for input."));
        }
    });
}

MacWindowCapture::~MacWindowCapture()
{
    stop();
}

void MacWindowCapture::start()
{
    stop();
#if __MAC_OS_X_VERSION_MAX_ALLOWED >= 140000
    if (@available(macOS 14.0, *)) {
        if (activeBridge) {
            emit failed(tr("Another dashboard is sharing a macOS window. Release it before choosing another."));
            return;
        }
        auto *bridge = [[DockpipeWindowCaptureBridge alloc] init];
        bridge->delivery = std::make_shared<FrameDelivery>();
        bridge->delivery->owner = this;
        bridge->waiting = YES;
        bridge->framesQueue = dispatch_queue_create("com.dockpipe.window-preview", DISPATCH_QUEUE_SERIAL);
        m_state->bridge = bridge;
        activeBridge = bridge;
        SCContentSharingPicker *picker = SCContentSharingPicker.sharedPicker;
        SCContentSharingPickerConfiguration *configuration = [[SCContentSharingPickerConfiguration alloc] init];
        configuration.allowedPickerModes = SCContentSharingPickerModeSingleWindow;
        configuration.allowsChangingSelectedContent = NO;
        picker.defaultConfiguration = configuration;
        [picker addObserver:bridge];
        picker.active = YES;
        emit statusChanged(tr("Choose one window in the macOS sharing picker. Sharing is view-only."));
        m_state->deadline.start(120000);
        [picker presentPickerUsingContentStyle:SCShareableContentStyleWindow];
        return;
    }
#endif
    emit failed(tr("Window sharing requires macOS 14 or later and a build using the macOS 14+ ScreenCaptureKit SDK."));
}

void MacWindowCapture::stop()
{
    m_state->deadline.stop();
    m_state->hiddenDeadlineRemaining = -1;
#if __MAC_OS_X_VERSION_MAX_ALLOWED >= 140000
    if (@available(macOS 14.0, *)) {
        DockpipeWindowCaptureBridge *bridge = m_state->bridge;
        if (!bridge)
            return;
        bridge->delivery->active = false;
        bridge->delivery->owner = nullptr;
        {
            std::lock_guard<std::mutex> lock(bridge->delivery->mutex);
            bridge->delivery->latest = {};
        }
        [SCContentSharingPicker.sharedPicker removeObserver:bridge];
        if (activeBridge == bridge) {
            activeBridge = nil;
            SCContentSharingPicker.sharedPicker.active = NO;
        }
        SCStream *stoppingStream = bridge->stream;
        bridge->stream = nil;
        if (stoppingStream) {
            [stoppingStream removeStreamOutput:bridge type:SCStreamOutputTypeScreen error:nil];
            [stoppingStream stopCaptureWithCompletionHandler:^(NSError *) {
                // Retain the bridge until ScreenCaptureKit finishes asynchronous
                // shutdown; never wait on the Qt UI thread or retain image queues.
                (void)bridge;
                (void)stoppingStream;
            }];
        }
        m_state->bridge = nil;
    }
#endif
}

void MacWindowCapture::setVisible(bool visible)
{
#if __MAC_OS_X_VERSION_MAX_ALLOWED >= 140000
    if (@available(macOS 14.0, *)) {
        DockpipeWindowCaptureBridge *bridge = m_state->bridge;
        if (!bridge)
            return;
        bridge->delivery->visible = visible;
        if (!visible) {
            std::lock_guard<std::mutex> lock(bridge->delivery->mutex);
            bridge->delivery->latest = {};
        }
        // Keep the finite chooser deadline while consent is pending. Only the
        // granted stream's wait for its first visible frame is suspended.
        if (!visible && !bridge->waiting && m_state->deadline.isActive()) {
            m_state->hiddenDeadlineRemaining = m_state->deadline.remainingTime();
            m_state->deadline.stop();
        } else if (visible && m_state->hiddenDeadlineRemaining >= 0) {
            m_state->deadline.start(qMax(1, m_state->hiddenDeadlineRemaining));
            m_state->hiddenDeadlineRemaining = -1;
        }
        [bridge updateVisibilityConfiguration];
    }
#else
    Q_UNUSED(visible);
#endif
}
