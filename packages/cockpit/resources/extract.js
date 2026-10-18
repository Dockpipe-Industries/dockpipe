(() => {
    // A completion callback from the previous document can race a navigation.
    // Wait for meaningful content, not for unrelated scripts after the article:
    // those can keep the parser loading until the corridor deadline expires.
    // One bounded walk. Only public rendered content and HTTP destinations cross
    // the isolated-world boundary; forms, cookies and page callbacks stay here.
    const limit = 12;
    const clean = (value, size) => String(value || "").slice(0, size * 4).replace(/\s+/g, " ").trim().slice(0, size);
    const github = location.hostname === "github.com";
    const searchHost = /(^|\.)(google\.(?:[a-z]{2,3}|(?:co|com)\.[a-z]{2})|bing\.com|duckduckgo\.com|search\.yahoo\.com)$/.test(location.hostname);
    const searchQuery = new URLSearchParams(location.search);
    const searchPage = searchHost && (searchQuery.has("q") || searchQuery.has("p"));
    if (document.readyState === "loading") {
        const root = document.querySelector('main,[role="main"],article,#search,#links');
        const heading = document.querySelector(searchPage ? 'h2 a[href],h3,a[data-testid="result-title-a"]' : 'h1');
        if (!heading || !root || root.textContent.slice(0, 8192).trim().length < 120)
            return JSON.stringify({pending: true});
    }
    const youtube = /(^|\.)youtube\.com$/.test(location.hostname);
    const candidates = [];
    const seen = new Map();
    const marketplace = /(^|\.)(amazon\.[a-z.]+|ebay\.[a-z.]+|etsy.com|walmart.com)$/.test(location.hostname);
    const pageHeading = document.querySelector("h1");
    const articleRoot = document.querySelector('[itemprop="articleBody"]')
        || (pageHeading && pageHeading.closest('article'));
    const articleMetadata = document.querySelector('meta[property="og:type"][content="article"]');
    const article = !!pageHeading && (!!articleMetadata || (!!articleRoot && articleRoot.contains(pageHeading)));
    const contentRoot = articleRoot || document.querySelector('main,[role="main"]') || document.body;
    const imageBounds = node => {
        const bounds = node.getBoundingClientRect();
        if (node.tagName !== 'IMG' || node.naturalWidth > 1 || (bounds.width >= 160 && bounds.height >= 90))
            return bounds;
        // Hydration may still have a one-pixel placeholder in a full-size image
        // frame. Reserve its exhibit now; the bounded capture pass supplies pixels.
        let parent = node.parentElement;
        for (let depth = 0; parent && depth < 2; depth++, parent = parent.parentElement) {
            const frame = parent.getBoundingClientRect();
            if (frame.width >= 160 && frame.height >= 90 && frame.width <= 1600 && frame.height <= 1200)
                return frame;
        }
        return bounds;
    };
    let productCount = 0;
    let vehicleCount = 0;
    const resultRoot = searchPage && document.querySelector('#rso,#b_results,#links,[data-testid="mainline"]');
    const walker = document.createTreeWalker(resultRoot || document.body || document.documentElement, NodeFilter.SHOW_ELEMENT);
    let visited = 0;
    let element;
    while (visited < 5000 && (element = walker.nextNode())) {
        visited++;
        const tag = element.tagName;
        const product = element.matches('[itemtype*="Product"],[itemtype*="Vehicle"],[itemtype*="Car"],[data-component-type="s-search-result"],.s-item');
        if (product)
            productCount++;
        if (element.matches('[itemtype*="Vehicle"],[itemtype*="Car"]'))
            vehicleCount++;
        if (tag !== "A" && tag !== "VIDEO" && tag !== "ARTICLE" && !product)
            continue;
        // Article worlds represent their own content, not unrelated links in
        // site-wide menus or recommendation rails elsewhere in the document.
        if (article && contentRoot && !contentRoot.contains(element))
            continue;
        if (article && element === articleRoot && !product)
            continue;
        const rect = element.getBoundingClientRect();
        if (rect.width < 24 || rect.height < 16 || element.closest('[hidden],[aria-hidden="true"]'))
            continue;
        const style = getComputedStyle(element);
        if (style.display === "none" || style.visibility === "hidden")
            continue;
        const image = element.querySelector("picture img,img");
        let artwork = image || element.querySelector("svg");
        if (!artwork) {
            // Decorative CSS artwork is common in hero/product cards. Inspect a
            // bounded set of direct children, never an arbitrary page screenshot.
            const decorations = [element, ...Array.from(element.children).slice(0, 6)];
            artwork = decorations.find(child => !clean(child.textContent, 1)
                && getComputedStyle(child).backgroundImage.startsWith("url(")) || null;
        }
        const artworkRect = artwork && artwork.getBoundingClientRect();
        const visualAspect = artworkRect && artworkRect.height > 0
            ? Math.max(0.5, Math.min(2.4, artworkRect.width / artworkRect.height)) : 1.6;
        const heading = element.querySelector("h1,h2,h3,h4");
        let title = clean(element.getAttribute("aria-label"), 140) || clean(heading && heading.textContent, 140)
            || clean(element.textContent, 140) || clean(image && image.alt, 140) || clean(element.getAttribute("title"), 140);
        if (!title && tag !== "VIDEO")
            continue;
        let href = "";
        let sourceHost = "";
        let link = tag === "A" ? element : null;
        if (!link && heading) {
            const headingLinks = heading.querySelectorAll("a[href]");
            if (headingLinks.length)
                link = headingLinks[headingLinks.length - 1];
        }
        if (!link && product)
            link = element.querySelector("a[href]");
        if (link) {
            try {
                let url = new URL(link.href, location.href);
                if (searchPage) {
                    const citation = link.querySelector('cite');
                    const source = clean(citation && citation.childNodes[0] && citation.childNodes[0].textContent, 200);
                    if (/^https?:\/\//.test(source)) {
                        try {
                            sourceHost = new URL(source).hostname;
                        } catch (_) {
                            // A malformed citation cannot identify an opaque result.
                            sourceHost = "";
                        }
                    }
                    // Favicons identify sources, but are not exhibit photographs.
                    if (image && image.getBoundingClientRect().width < 64)
                        artwork = null;
                }
                if (searchPage && url.hostname === location.hostname) {
                    let destination = url.searchParams.get("url") || url.searchParams.get("uddg")
                        || (url.pathname === "/url" ? url.searchParams.get("q") : null);
                    const encoded = url.searchParams.get("u");
                    if (!destination && /(^|\.)bing\.com$/.test(url.hostname) && encoded && encoded.startsWith("a1"))
                        destination = atob(encoded.slice(2).replace(/-/g, "+").replace(/_/g, "/"));
                    if (destination && /^https?:\/\//.test(destination))
                        url = new URL(destination);
                    // Google also supplies opaque /goto tokens. Keep its actual
                    // link and let ordinary browser navigation resolve it on entry;
                    // the visible citation is attribution, never a guessed URL.
                }
                // Search chrome, account controls and pagination are not places.
                const opaqueResult = searchPage && /(^|\.)google\.(?:[a-z]{2,3}|(?:co|com)\.[a-z]{2})$/.test(url.hostname)
                    && url.pathname === "/goto" && sourceHost && sourceHost !== url.hostname
                    && /^[A-Za-z0-9_=-]{1,2048}$/.test(url.searchParams.get("url") || "");
                if (searchPage && (!(heading || link.closest("h2,h3")) || (url.hostname === location.hostname && !opaqueResult)
                    || /(^|\.)(accounts\.google\.com|support\.google\.com)$/.test(url.hostname)))
                    continue;
                if (!/^https?:$/.test(url.protocol) || url.username || url.password || url.href.length > 4096)
                    continue;
                href = url.href;
                const source = new URL(location.href);
                source.hash = "";
                url.hash = "";
                if (url.href === source.href)
                    continue;
            } catch (_) {
                continue;
            }
        }
        if (/^(load more|show more|next|previous|search|shop|subscribe|sign in|log in)$/i.test(title))
            continue;
        const identity = href || title || "video";
        let kind = "link";
        if (product || (marketplace && /\/(dp|gp\/product|itm|listing)\//.test(href)))
            kind = "product";
        else if (tag === "VIDEO" || href.includes("/watch?") || element.closest('[itemtype*="VideoObject"]'))
            kind = "video";
        else if (github && href.startsWith("https://github.com/")) {
            const path = new URL(href).pathname;
            if (/^\/[^/]+\/[^/]+\/tree\//.test(path))
                kind = "folder";
            else if (/^\/[^/]+\/[^/]+\/blob\//.test(path))
                kind = "file";
            else if (/^\/[^/]+\/[^/]+\/?$/.test(path)
                && !/^\/(topics|collections|sponsors|settings|features|orgs)\//.test(path))
                kind = "repository";
        }
        // Only model-specific cards become cars. Service/navigation links and
        // arbitrary years elsewhere on the page cannot manufacture a vehicle.
        const vehicleText = clean(title + " " + (image && image.alt), 400);
        const vehiclePath = /\/(trucks|suvs|cars|vans|vehicles|motorcycles|dirt-bikes|off-road)\/[^/?#]+/i.test(href);
        const vehicleSchema = element.matches('[itemtype*="Vehicle"],[itemtype*="Car"]');
        const yearMatch = vehicleText.match(/\b(19[0-9]{2}|20[0-9]{2})\b/);
        let vehicleBody = "";
        let vehicleYear = 0;
        const utilityLink = /receive.*information|keep me informed|sign up|newsletter|brochure|apply for|financing|insurance|warranty|schedule|payment estimator/i.test(title);
        if (!utilityLink && (vehicleSchema || (vehiclePath && (image || yearMatch)))) {
            const bodyEvidence = vehicleText + " " + new URL(href || location.href).pathname;
            if (/dirt[ -]?bike|motocross|enduro|off[ -]?road motorcycle/i.test(bodyEvidence))
                vehicleBody = "dirt-bike";
            else if (/pickup|pick-up|f[- ]?150|f[- ]?[234]50|maverick|ranger|\/trucks\//i.test(bodyEvidence))
                vehicleBody = "pickup";
            else if (/\bvan\b|transit|\/vans\//i.test(bodyEvidence))
                vehicleBody = "van";
            else if (/\bsuv\b|crossover|mach-e|bronco|explorer|escape|expedition|\/suvs\//i.test(bodyEvidence))
                vehicleBody = "suv";
            else if (/coupe|convertible|mustang|sports car/i.test(bodyEvidence))
                vehicleBody = "coupe";
            else
                vehicleBody = "sedan";
            vehicleYear = yearMatch ? Number(yearMatch[1]) : 0;
            kind = "vehicle";
            vehicleCount++;
            const imageYear = image && image.alt.match(/\b(19[0-9]{2}|20[0-9]{2})\b/);
            if (imageYear && Number(imageYear[1]) === vehicleYear)
                title = clean(image.alt, 140);
            title = title.replace(/^(learn more (about )?|view (the )?)/i, "").replace(/ model details$/i, "");
        }
        const main = !!element.closest("main,[role=main],article");
        // Many sites implement their header entirely with divs. An image in
        // that chrome must never outrank a real content card further down.
        const navigation = !!element.closest('nav,header,footer,[role=navigation],[role=banner],[class*="NavBar"],[class*="navbar"],[class*="Navigation"],[class*="Footer"]');
        if (navigation)
            continue;
        const semantic = kind !== "link";
        const score = (artwork ? 10 : 0) + (heading ? 5 : 0) + (main ? 5 : 0)
            + (semantic ? 40 : 0) + (tag === "ARTICLE" ? 20 : 0);
        const description = element.querySelector('p,[itemprop="description"]');
        const price = element.querySelector('[itemprop="price"],.a-price .a-offscreen');
        const section = element.closest("section,[aria-label],article");
        const sectionHeading = section && section.querySelector("h2,h3");
        const category = clean((section && section.getAttribute("aria-label")) || (sectionHeading && sectionHeading.textContent), 60);
        const prior = seen.get(identity);
        if (prior && prior.score >= score)
            continue;
        // Capture only an actual image. Never turn an article or entire page into
        // a screenshot panel: text becomes native signage and links become places.
        const candidate = {element, vehicleBody, vehicleYear, capture: artwork, visualAspect, title: title || "Play video", href, score, kind, category,
            description: clean(description && description.textContent, 200),
            detail: searchPage ? (sourceHost || new URL(href || location.href).hostname) : clean(price && price.textContent, 60)};
        if (prior)
            candidates[candidates.indexOf(prior)] = candidate;
        else
            candidates.push(candidate);
        seen.set(identity, candidate);
    }
    if (article && contentRoot) {
        // A source article is content in its own right, even without outbound
        // links. Preserve bounded excerpts and images instead of its site chrome.
        let excerpts = 0;
        let illustrations = 0;
        let textNodesRead = 0;
        const articleChrome = node => {
            if (node.closest('nav,header,footer,aside,figure,figcaption,button,[hidden],[aria-hidden="true"],[aria-roledescription="carousel"]'))
                return true;
            let parent = node;
            for (let depth = 0; parent && depth < 64; depth++, parent = parent.parentElement) {
                if (/(?:^|[\s_-])(copyright|caption|credit|credits)(?:$|[\s_-])/i.test(parent.getAttribute('class') || ''))
                    return true;
            }
            return false;
        };
        const prose = node => {
            if (articleChrome(node))
                return '';
            const textWalker = document.createTreeWalker(node, NodeFilter.SHOW_TEXT);
            let text = '';
            let part;
            while (text.length < 800 && textNodesRead < 2048 && (part = textWalker.nextNode())) {
                textNodesRead++;
                const parent = part.parentElement;
                if (!parent || articleChrome(parent) || !parent.getClientRects().length)
                    continue;
                const style = getComputedStyle(parent);
                if (style.visibility !== 'hidden' && style.display !== 'none')
                    text += part.textContent.slice(0, 800 - text.length);
            }
            return clean(text, 200);
        };
        const excerptTexts = new Set();
        const content = contentRoot.querySelectorAll("p,img");
        for (let index = 0; index < Math.min(content.length, 256); index++) {
            const node = content[index];
            if (node.closest('nav,header,footer,[hidden],[aria-hidden="true"]')
                || !node.getClientRects().length || getComputedStyle(node).visibility === 'hidden')
                continue;
            const rect = imageBounds(node);
            if (rect.width < 24 || rect.height < 16)
                continue;
            const text = node.tagName === "P" && excerpts < 4 ? prose(node) : '';
            if (text.length >= 80 && excerpts < 4 && !excerptTexts.has(text)) {
                excerpts++;
                excerptTexts.add(text);
                candidates.push({element: node, capture: null, title: clean(pageHeading.textContent, 100) + " · " + excerpts,
                    href: location.href, kind: "excerpt", score: 100, description: text, category: "Read"});
            } else if (node.tagName === "IMG" && rect.width >= 160 && rect.height >= 90 && illustrations < 4) {
                const title = clean(node.alt, 140);
                if (!title || candidates.some(item => item.capture === node))
                    continue;
                illustrations++;
                candidates.push({element: node, capture: node, title, href: "", kind: "image", score: 90,
                    visualAspect: Math.max(0.5, Math.min(2.4, rect.width / rect.height)), category: "Explore"});
            }
        }
    }
    candidates.sort((a, b) => b.score - a.score);
    const description = document.querySelector('meta[name="description"],meta[property="og:description"]');
    const evidence = clean(document.title, 180) + " " + clean(pageHeading && pageHeading.textContent, 300)
        + " " + clean(description && description.content, 500)
        + " " + candidates.slice(0, 8).map(item => item.title).join(" ");
    const text = evidence.toLowerCase();
    let venue = github ? "repositories" : "mall";
    const video = document.querySelector("main video,[role=main] video");
    if (youtube || (!article && !marketplace && !productCount && video)) {
        venue = "cinema";
    } else if (vehicleCount || (/dealership|vehicle inventory|used cars|new cars/.test(text) && /vehicle|car|motor|test drive/.test(text))) {
        venue = "showroom";
    } else if (marketplace || productCount) {
        venue = "store";
    } else if (/documentation|developer guide|api reference|manual|knowledge base/.test(text)) {
        venue = "library";
    } else if (/rain[ -]?forest|wildlife|habitat|animal|invertebrate|mammal|amphibian|reptile|woodland|nature/.test(text)) {
        venue = "park";
    }
    if (searchPage)
        venue = "city";
    // A theatre reserves one texture for the playing film. Prefer actual videos
    // and products over site chrome without treating arbitrary page text as code.
    let selected = candidates.slice(0, venue === "cinema" ? 11 : limit);
    if (venue === "cinema") {
        const videos = candidates.filter(item => item.kind === "video");
        if (videos.length)
            selected = videos.slice(0, 11);
    }
    if (venue === "store") {
        const products = candidates.filter(item => item.kind === "product");
        if (products.length)
            selected = products.slice(0, limit);
        selected.sort((a, b) => a.category.localeCompare(b.category));
    }
    if (venue === "showroom") {
        const vehicles = candidates.filter(item => item.kind === "vehicle");
        if (vehicles.length)
            selected = vehicles.slice(0, limit);
    }
    // A heading may arrive before the actual exhibits. Keep waiting instead of
    // freezing a partially parsed document that has no useful content yet.
    if (!selected.length && document.readyState === "loading")
        return JSON.stringify({pending: true});
    const theme = document.querySelector('meta[name="theme-color"]');
    let accent = theme && /^#[0-9a-f]{6}$/i.test(theme.content) ? theme.content : "";
    if (!accent && selected.length) {
        const rgb = getComputedStyle(selected[0].element).color.match(/^rgb\((\d+),\s*(\d+),\s*(\d+)\)$/);
        if (rgb) {
            const channels = rgb.slice(1).map(Number);
            if (Math.max(...channels) - Math.min(...channels) > 35)
                accent = "#" + channels.map(channel => channel.toString(16).padStart(2, "0")).join("");
        }
    }
    window.__dockpipeSpatial = {nodes: selected, scrollX, scrollY};
    return JSON.stringify({title: clean(document.title, 180), url: location.href.slice(0, 4096), venue, evidence, accent,
        limited: candidates.length > selected.length || visited >= 5000,
        items: selected.map((item, index) => ({id: String(index), title: item.title, href: item.href,
            kind: item.kind, vehicleBody: item.vehicleBody, vehicleYear: item.vehicleYear, description: item.description, category: item.category,
            detail: item.detail, visualAspect: item.visualAspect, hasImage: !!item.capture}))});
})()
