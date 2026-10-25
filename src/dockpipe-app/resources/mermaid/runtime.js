"use strict";

window.dockpipeResult = null;
window.dockpipeRender = async function (source, dark) {
    try {
        mermaid.initialize({
            startOnLoad: false,
            securityLevel: "strict",
            theme: dark ? "dark" : "default",
            maxTextSize: 50000,
            maxEdges: 500,
            htmlLabels: false,
            suppressErrorRendering: true,
            secure: ["secure", "securityLevel", "startOnLoad", "maxTextSize", "maxEdges", "htmlLabels", "suppressErrorRendering"]
        });
        const result = await mermaid.render("dockpipeDiagram", source);
        if (result.svg.length > 4 * 1024 * 1024)
            throw new Error("Diagram output is too large.");

        // Mermaid may emit xlink:href without its namespace declaration. Add
        // the declaration before XML parsing; all links are removed below.
        const serializedSvg = /<svg\b[^>]*\bxmlns:xlink\s*=/.test(result.svg)
            ? result.svg
            : result.svg.replace(/<svg\b/, '<svg xmlns:xlink="http://www.w3.org/1999/xlink"');
        const document = new DOMParser().parseFromString(serializedSvg, "image/svg+xml");
        const svg = document.documentElement;
        if (svg.localName !== "svg" || document.querySelector("parsererror"))
            throw new Error("Diagram did not produce valid SVG.");

        // Strict Mermaid is the first boundary; remove interactive and remote SVG
        // content again before handing its output to the browser image decoder.
        for (const element of [...svg.querySelectorAll("script,foreignObject,image,use,iframe,object,embed")])
            element.remove();
        for (const anchor of [...svg.querySelectorAll("a")])
            anchor.replaceWith(...anchor.childNodes);
        for (const element of [svg, ...svg.querySelectorAll("*")]) {
            for (const attribute of [...element.attributes]) {
                const name = attribute.name.toLowerCase();
                if (name.startsWith("on") || name.endsWith("href") || /url\s*\(\s*["']?(?!#)/i.test(attribute.value))
                    element.removeAttribute(attribute.name);
            }
        }
        for (const style of svg.querySelectorAll("style")) {
            style.textContent = style.textContent.replace(/@import[^;]*;?/gi, "")
                .replace(/url\s*\(\s*(["']?)([^)]*?)\1\s*\)/gi,
                    (match, quote, target) => target.trim().startsWith("#") ? match : "none");
        }

        const bounds = svg.viewBox.baseVal;
        let width = Math.ceil(bounds.width || Number.parseFloat(svg.getAttribute("width")));
        let height = Math.ceil(bounds.height || Number.parseFloat(svg.getAttribute("height")));
        if (!Number.isFinite(width) || !Number.isFinite(height) || width <= 0 || height <= 0)
            throw new Error("Diagram has invalid dimensions.");
        const scale = Math.min(1, 4096 / width, 4096 / height, Math.sqrt(4000000 / (width * height)));
        width = Math.max(1, Math.floor(width * scale));
        height = Math.max(1, Math.floor(height * scale));
        svg.setAttribute("width", String(width));
        svg.setAttribute("height", String(height));
        const serialized = new XMLSerializer().serializeToString(svg);
        const picture = new Image();
        picture.onload = function () {
            try {
                const canvas = window.document.createElement("canvas");
                canvas.width = width;
                canvas.height = height;
                const context = canvas.getContext("2d");
                context.fillStyle = dark ? "#20242b" : "#ffffff";
                context.fillRect(0, 0, width, height);
                context.drawImage(picture, 0, 0, width, height);
                const data = canvas.toDataURL("image/png");
                if (data.length > 22 * 1024 * 1024)
                    throw new Error("Diagram image is too large.");
                window.dockpipeResult = { png: data };
            } catch (error) {
                window.dockpipeResult = { error: String(error.message || error).slice(0, 1000) };
            }
        };
        picture.onerror = function () {
            window.dockpipeResult = { error: "Diagram image could not be decoded." };
        };
        picture.src = "data:image/svg+xml;charset=utf-8," + encodeURIComponent(serialized);
    } catch (error) {
        window.dockpipeResult = { error: String(error.message || error).slice(0, 1000) };
    }
};
