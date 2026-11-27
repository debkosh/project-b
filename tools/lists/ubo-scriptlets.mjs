// uBlock Origin's scriptlets as adblock-rust's resources, made as Brave makes them
// (brave/brave-core-crx-packager, lib/adBlockRustUtils.js): each of uBO's built-in scriptlets
// is its function's source, base64, with its aliases and the names of the functions it needs.
// adblock-rust's own reader of scriptlets.js reads only uBO's old template format, so this is
// the part done in JavaScript; core/blocking/lib/examples/resources.rs puts the rest together.
//
//	node tools/lists/ubo-scriptlets.mjs <uBlock Origin checkout> > ubo-scriptlets.json
import path from 'node:path';
import { pathToFileURL } from 'node:url';

const root = process.argv[2];
if (!root) {
    console.error('usage: node ubo-scriptlets.mjs <uBlock Origin checkout>');
    process.exit(2);
}
const { builtinScriptlets } = await import(pathToFileURL(path.resolve(root, 'src/js/resources/scriptlets.js')).href);

const names = new Set(builtinScriptlets.map(s => s.name));
const out = builtinScriptlets.map(s => {
    for (const dep of s.dependencies ?? []) {
        if (!names.has(dep)) {
            throw new Error(`uBO scriptlet ${s.name} needs ${dep}, which isn't there`);
        }
    }
    return {
        name: s.name,
        aliases: s.aliases ?? [],
        kind: { mime: 'application/javascript' },
        content: Buffer.from(s.fn.toString()).toString('base64'),
        dependencies: s.dependencies ?? [],
    };
});
if (out.length === 0) {
    throw new Error('no scriptlets');
}
process.stdout.write(JSON.stringify(out) + '\n');
