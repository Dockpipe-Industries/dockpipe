# KSyntaxHighlighting 5.116.0

Upstream: https://invent.kde.org/frameworks/syntax-highlighting
Source: https://download.kde.org/stable/frameworks/5.116/syntax-highlighting-5.116.0.tar.xz
SHA256: `6f8ccacebd0dc9ce050b2b23b715750c97938b3336fbbb80967920cb8c1dfaa7`

The native highlighting engine and Breeze themes are MIT licensed. Copyright
notices are preserved in the source archive. The separately distributed
`dockpipe-syntax.rcc` contains upstream syntax definitions under their individual
licenses (MIT, LGPL, GPL and others, as declared in each definition). Applicable
license texts accompany that resource. Complete corresponding source, including
the definitions and notices, is included in this source archive in the Dockpipe
source repository. The archive is unmodified; the adjacent CMake adapter replaces
KDE's build tooling and excludes downloading, network, QML and command-line tools.
The PipeLang definition is Dockpipe-authored under the repository Apache-2.0 license.

This release is pinned to preserve the launcher's Qt 6.2 build compatibility.
Updates must verify the archive hash, Qt compatibility, language fixtures, package
contents and notices. Builds do not download dependencies or syntax definitions.

The adjacent `syntax/*-php.xml` and `syntax/doxygenlua.xml` files are generated
from the unmodified archive using its Perl generators. They are checked in to
avoid a Perl dependency on end-user build machines:

    perl data/generators/generate-php.pl data/syntax/<name>.xml syntax/<name>-php.xml
    perl data/generators/generate-doxygenlua.pl data/syntax/doxygen.xml syntax/doxygenlua.xml

The PHP names are javascript, css, html, javascript-react, typescript and mustache.
Their original notices and licenses are retained in the generated files.
