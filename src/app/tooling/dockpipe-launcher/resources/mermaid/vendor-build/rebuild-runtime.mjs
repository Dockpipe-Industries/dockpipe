import { createHash } from "node:crypto";
import { readFileSync, renameSync, statSync, writeFileSync } from "node:fs";
import { createRequire } from "node:module";
import { dirname, resolve } from "node:path";

const [inputArgument, dependenciesArgument, outputArgument] = process.argv.slice(2);
if (!inputArgument || !dependenciesArgument || !outputArgument) {
    throw new Error("Usage: node rebuild-runtime.mjs upstream-mermaid.min.js dependencies-directory output.js");
}

const input = resolve(inputArgument);
const dependencies = resolve(dependenciesArgument);
const output = resolve(outputArgument);
const require = createRequire(resolve(dependencies, "package.json"));
const esbuild = require("esbuild");
const coreVersion = require("core-js/package.json").version;
if (esbuild.version !== "0.25.12" || coreVersion !== "3.45.1")
    throw new Error("Use the locked esbuild 0.25.12 and core-js 3.45.1 build dependencies.");

if (statSync(input).size > 6 * 1024 * 1024)
    throw new Error("The pinned Mermaid input exceeds its 6 MiB build limit.");
const upstream = readFileSync(input);
const upstreamHash = createHash("sha256").update(upstream).digest("hex");
if (upstream.length > 6 * 1024 * 1024
    || upstreamHash !== "a8a6a6f805b074a8261a2825c86f99c8054acb3b35f50bb7e24a4f9f2c3c3b9f")
    throw new Error("Input must be the pinned, unmodified Mermaid 12.2.0 standalone distribution.");

// Qt 6.2 embeds Chromium 90. Syntax lowering does not supply newer standard
// APIs, so bundle only those used by the pinned Mermaid distribution.
const polyfillModules = [
    "core-js/actual/object/has-own",
    "core-js/actual/object/group-by",
    "core-js/actual/array/at",
    "core-js/actual/string/at",
    "core-js/actual/structured-clone"
];
const polyfills = esbuild.buildSync({
    stdin: {
        contents: polyfillModules.map(name => `import ${JSON.stringify(name)};`).join("\n"),
        resolveDir: dependencies
    },
    bundle: true,
    target: "chrome90",
    minify: true,
    legalComments: "inline",
    sourcemap: false,
    write: false
}).outputFiles[0].contents;
const transformed = esbuild.transformSync(upstream.toString("utf8"), {
    target: "chrome90",
    // Escape literal newlines and trailing spaces without changing their value.
    supported: { "template-literal": false },
    minify: true,
    legalComments: "inline",
    sourcemap: false,
    logLevel: "silent"
}).code;
const coreLicense = readFileSync(resolve(dirname(require.resolve("core-js/package.json")), "LICENSE"), "utf8");
const result = Buffer.concat([
    Buffer.from("/* Mermaid 12.2.0 with Chromium 90 compatibility; see PROVENANCE.md and LICENSE.core-js. */\n"),
    Buffer.from("/*\n" + coreLicense + "\n*/\n"),
    polyfills,
    Buffer.from("\n" + transformed)
]);
if (result.length > 8 * 1024 * 1024 || result.includes(Buffer.from(dependencies)))
    throw new Error("Generated runtime is oversized or contains a private build path.");
writeFileSync(output + ".tmp", result);
renameSync(output + ".tmp", output);
console.log(JSON.stringify({ bytes: result.length, sha256: createHash("sha256").update(result).digest("hex") }));
