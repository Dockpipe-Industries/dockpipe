Dockpipe's macOS and Windows desktop distributions dynamically link Qt 6.10.3
(Core, Gui, Widgets, Network, Concurrent and runtime plugins). Qt is copyright
The Qt Company Ltd. and other contributors, and is used under the GNU Lesser
General Public License version 3. The LGPL and referenced GPL version 3 texts
are included here.

The corresponding Qt source is available at:
https://download.qt.io/archive/qt/6.10/6.10.3/single/

Dockpipe does not modify Qt. These shared libraries may be replaced with
compatible builds; debugging modifications to the LGPL libraries is permitted.
Apple signing or notarization of a modified bundle must be handled by the person
making the replacement. Linux ARM64 packages obtain Qt from the distribution's
package manager and use its license and source packages. Linux amd64 packages
ship Qt 6.10.3 with a separate version-specific README and upstream SPDX license
inventories under /usr/share/doc/dockpipe-desktop/qt.

Dockpipe source and build instructions:
https://github.com/Dockpipe-Industries/dockpipe
