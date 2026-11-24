# Desktop UI translations

Dockpipe uses Qt's native translation system: `tr()` and explicit translation
contexts in C++, editable Linguist `.ts` catalogs, and embedded `.qm` resources
compiled by `lrelease`. No translation JSON files are loaded at runtime.

Settings → General → Appearance → Language offers the system language or an
explicit choice. Saving a changed language offers **Restart now** or **Later**.
Restart now runs the normal unsaved-work checks and reopens Dockpipe automatically;
cancelling a work prompt leaves the app open and keeps the language preference.
Open terminal sessions close during restart. Theme preview remains immediate;
cancelling settings does not save a language change. Unsupported system languages
fall back to English.

The catalogs cover English, Spanish, French, German, Brazilian Portuguese,
Italian, Dutch, Polish, Russian, Ukrainian, Turkish, Arabic, Hindi, Japanese,
Korean, Simplified Chinese, and Traditional Chinese. Regional variants fall back
to a supported base language; Chinese script selects the corresponding catalog.
Arabic uses right-to-left UI layout; source editors and diffs retain left-to-right
layout for code.

The non-English catalogs are generated translations and need native-speaker
review. A finished entry means that the translation is present and passes the
structural checks; it does not indicate human linguistic approval.

## Maintaining catalogs

Install the matching Qt SDK's Linguist tools (`Qt6::LinguistTools`, `lupdate`, and
`lrelease`). On Debian/Ubuntu, the development dependencies include
`qt6-tools-dev` and `qt6-l10n-tools`.

From an already configured app build directory:

```sh
cmake --build /path/to/build --target dockpipe-translations-update
```

This extracts app-owned C++/Objective-C++ UI strings across platform branches.
Translate new or unfinished entries with Qt Linguist. Preserve `%1`, `%L1`, `%n`,
HTML tags, line breaks, keyboard mnemonics, commands, protocol identifiers,
environment variables, and file patterns. Supply every plural form required by
the catalog's locale. Keep user data and stable machine identifiers untranslated.

```sh
cmake --build /path/to/build --target dockpipe-launcher launcher-translations-test launcher-settings-test
ctest --test-dir /path/to/build --output-on-failure \
  -R '^(translation-catalogs|launcher-translations|launcher-settings)$'
```

The catalog check verifies source extraction, identical context/message sets,
completion, plural counts, placeholders, markup, line breaks, and mnemonics.
The Qt tests check embedded resources, locale fallback, settings persistence and
cancellation, Arabic startup direction, and code editor direction.
On Unix builds, the `language-restart` test also covers Restart now, Later, and
cancelling restart to preserve an unsaved workflow. `launcher-environment` checks
that reopening preserves operator overrides without inheriting app-set defaults.

Application catalogs are required and embedded in the shared UI library and
workspace-reader helper. Qt's matching standard-dialog catalogs are bundled when
provided by the SDK. Because the selected Qt SDK has no Hindi `qtbase` catalog,
`qtbase_hi.ts` supplies standard button labels only, rather than a complete Qt
framework translation. Native OS dialogs, external browser SDK UI, package/user
content, and raw CLI/tool output retain their owning component's language.

Commit authored `.ts` files, not generated `.qm` files. The build compiles the
catalogs automatically; it excludes unfinished translations so Qt can fall back
to the English source when developing a newly added entry.
