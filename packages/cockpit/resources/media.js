(() => {
    const video = document.querySelector("video");
    if (!video)
        return {found: false};
    if (video.__dockpipeStyle === undefined) {
        video.__dockpipeStyle = video.style.cssText;
        video.scrollIntoView({block: "center", behavior: "instant"});
        video.style.objectFit = "contain";
    }
    const rect = video.getBoundingClientRect();
    return {found: true, ready: video.readyState, time: video.currentTime,
        paused: video.paused, ended: video.ended,
        error: window.__dockpipeMediaError || (video.error ? "Media error " + video.error.code : ""),
        x: rect.x, y: rect.y, width: rect.width, height: rect.height};
})()
