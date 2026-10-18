cmake_minimum_required(VERSION 3.21.1)
# Built against the matching launcher UI SDK. The launcher does not link this target.
if(NOT TARGET dockpipe-launcher-ui)
    message(FATAL_ERROR "Build with the matching launcher SDK using DOCKPIPE_COCKPIT_SOURCE")
endif()
find_package(Qt6 REQUIRED COMPONENTS OpenGLWidgets)
find_package(ForgePipe 0.1.0 EXACT CONFIG REQUIRED)
option(DOCKPIPE_SPATIAL_WEB "Build the package's spatial browser" ON)
option(DOCKPIPE_APP_BILLBOARDS "Build the package's native app streaming" ON)
if(DOCKPIPE_SPATIAL_WEB)
    # Chromium 90 in Qt 6.2 cannot parse current site application bundles.
    # Use a matching modern launcher SDK, not a second embedded browser engine.
    set(QT_PROMOTE_TO_GLOBAL_TARGETS ON)
    find_package(Qt6 6.8 REQUIRED COMPONENTS WebEngineWidgets)
endif()
set(COCKPIT_SOURCES
    src/CockpitInputMenu.cpp
    src/CockpitInput.cpp
    src/ScrollingPreview.cpp
    src/SpatialSiteExplorer.cpp
    src/SpatialSitePolicy.cpp
    src/SpatialSitePage.cpp
    src/SpatialSiteWorld.cpp
    src/SpatialSiteFetch.cpp
    src/SpatialSiteCrawler.cpp
    src/FurnishingCatalog.cpp
    src/FurnishingPreview.cpp
    src/SpatialAssetLibrary.cpp
    src/FurnishingLayout.cpp
    src/FurnishingController.cpp
    src/ScreenProviders.cpp
    src/GitReviewModel.cpp
    src/GitReviewProgress.cpp
    src/GitReviewWorld.cpp
    src/GitReviewWidget.cpp
    src/WorkspaceFlightDynamics.cpp
    src/CockpitCommandDeck.cpp
    src/CockpitController.cpp
    src/CockpitHudPanel.cpp
    src/CockpitVoice.cpp
    src/CockpitWidget.cpp
    src/CockpitWorkspaces.cpp
    src/CockpitOperations.cpp
    src/SpatialOperationsWorld.cpp
    src/DesktopAppCatalog.cpp
    src/FlightAppBillboards.cpp
    src/FlightPlanetArt.cpp
    src/SpatialDisplayDialog.cpp
    src/SpatialInteriorGeometry.cpp
    src/DirtBikeSound.cpp
    src/DirtBikeDust.cpp
    src/DirtBikeCrashPose.cpp
    src/DirtBikeModel.cpp
    src/SpatialMeshLoft.cpp
    src/DirtBikeRiderPose.cpp
    src/DirtBikeSteeringRig.cpp
    src/DirtBikeTrackArt.cpp
    src/SpatialFoliagePlacement.cpp
    src/DirtBikeDynamics.cpp
    src/DirtBikeRaceCourse.cpp
    src/DirtBikeRaceProgress.cpp
    src/DirtBikeRivalDriver.cpp
    src/DirtBikeRace.cpp
    src/SpatialReturnPortal.cpp
    src/SpatialRideSurface.cpp
    src/RedBudCourse.cpp
    src/RedBudTerrain.cpp
    src/WorkspaceFlightTerrain.cpp
    src/SpatialBikeTrack.cpp
    src/SpatialVehicle.cpp
    src/SpatialAnimalModel.cpp
    src/SpatialWebPassage.cpp
    src/SpatialWebBrowser.cpp
    src/SpatialWebSearch.cpp
    src/SpatialWebIntelligence.cpp
    src/SpatialWebDocument.cpp
    src/SpatialWebGraph.cpp
    src/SpatialWebMedia.cpp
    src/SpatialCivicWorld.cpp
    src/WorkspaceFlightActions.cpp
    src/SpatialWebWorld.cpp
    src/SpatialWorkspace.cpp
    src/SpatialWorkspaceEditor.cpp
    src/SpatialWorkspaceStore.cpp
    src/SpatialWorldCompiler.cpp
    src/SpatialWorldPlanner.cpp
    src/SpatialWorldArchitecture.cpp
    src/SpatialNeuralService.cpp
    src/WorkspaceFlightCockpit.cpp
    src/WorkspaceFlightGpu.cpp
    src/WorkspaceFlightInterior.cpp
    src/WorkspaceFlightOrbit.cpp
    src/WorkspaceFlightVehicle.cpp
    src/WorkspaceFlightView.cpp
    src/WorkspaceGraphNode.cpp
    src/WorkspaceGraphPage.cpp
    src/WorkspaceGraphReader.cpp
    src/WorkspaceGraphView.cpp
    src/WorkspaceGraphWidget.cpp
)
add_library(dockpipe-cockpit SHARED ${COCKPIT_SOURCES} src/CockpitPackage.cpp cockpit.qrc models.qrc)
set_target_properties(dockpipe-cockpit PROPERTIES WINDOWS_EXPORT_ALL_SYMBOLS ON
    LIBRARY_OUTPUT_DIRECTORY "${CMAKE_CURRENT_BINARY_DIR}/assets/ui"
    RUNTIME_OUTPUT_DIRECTORY "${CMAKE_CURRENT_BINARY_DIR}/assets/ui"
    BUILD_RPATH "$ORIGIN/../lib" BUILD_RPATH_USE_ORIGIN TRUE
    INSTALL_RPATH "$ORIGIN/../lib")
if(CMAKE_SYSTEM_NAME STREQUAL "Linux")
    # A package-local RPATH also resolves the browser runtime's transitive libraries.
    target_link_options(dockpipe-cockpit PRIVATE "LINKER:--disable-new-dtags")
endif()
target_include_directories(dockpipe-cockpit PUBLIC "${CMAKE_CURRENT_SOURCE_DIR}/src")
target_link_libraries(dockpipe-cockpit PRIVATE dockpipe-launcher-ui dockpipe-browser Qt6::OpenGLWidgets PUBLIC ForgePipe::Engine)
if(DOCKPIPE_SPATIAL_WEB)
    target_compile_definitions(dockpipe-cockpit PUBLIC DOCKPIPE_SPATIAL_WEB)
    target_link_libraries(dockpipe-cockpit PUBLIC Qt6::WebEngineWidgets)
endif()
if(DOCKPIPE_APP_BILLBOARDS AND CMAKE_SYSTEM_NAME STREQUAL "Linux")
    find_package(X11 QUIET)
    find_library(DOCKPIPE_XCB_LIBRARY xcb)
    if(X11_Xcomposite_FOUND AND X11_Xrender_FOUND AND DOCKPIPE_XCB_LIBRARY)
        target_compile_definitions(dockpipe-cockpit PUBLIC DOCKPIPE_X11_BILLBOARDS)
        target_link_libraries(dockpipe-cockpit PUBLIC X11::X11 X11::Xcomposite X11::Xrender ${DOCKPIPE_XCB_LIBRARY})
    endif()
endif()
add_executable(cockpit-reader src/CockpitReaderMain.cpp src/WorkspaceGraphPage.cpp)
target_include_directories(cockpit-reader PRIVATE "${DOCKPIPE_LAUNCHER_SOURCE_DIR}/src")
target_link_libraries(cockpit-reader PRIVATE Qt6::Core)
set_target_properties(cockpit-reader PROPERTIES RUNTIME_OUTPUT_DIRECTORY "${CMAKE_CURRENT_BINARY_DIR}/assets/ui"
    BUILD_RPATH "$ORIGIN/../lib" BUILD_RPATH_USE_ORIGIN TRUE)
add_dependencies(dockpipe-cockpit cockpit-reader)

if(BUILD_TESTING)
    add_executable(cockpit-package-rendering-test tests/PackageRenderingTest.cpp)
    target_link_libraries(cockpit-package-rendering-test PRIVATE dockpipe-launcher-ui)
    set_target_properties(cockpit-package-rendering-test PROPERTIES RUNTIME_OUTPUT_DIRECTORY "${CMAKE_BINARY_DIR}")
    add_dependencies(cockpit-package-rendering-test dockpipe-cockpit)
    add_test(NAME cockpit-package-rendering COMMAND cockpit-package-rendering-test)
    set_tests_properties(cockpit-package-rendering PROPERTIES
        ENVIRONMENT "QT_QPA_PLATFORM=offscreen;SDL_AUDIODRIVER=dummy;DOCKPIPE_WORKSPACE_PACKAGE=${CMAKE_CURRENT_BINARY_DIR}/assets"
        TIMEOUT 30 SKIP_RETURN_CODE 77)
    add_executable(cockpit-test tests/CockpitTest.cpp tests/SpatialCockpitTest.cpp tests/InputControlsTest.cpp
        tests/SpatialCivicTest.cpp tests/SpatialSiteTest.cpp tests/FurnishingTest.cpp tests/GitReviewTest.cpp tests/FlightDynamicsTest.cpp tests/SpatialOperationsTest.cpp tests/NativeHandoffTest.cpp tests/SpatialIntelligenceTest.cpp tests/SpatialWebTest.cpp tests/SpatialVehicleTest.cpp tests/DirtBikeTest.cpp tests/DirtBikeRaceTest.cpp tests/SpatialTerrainTest.cpp tests/ReturnPortalTest.cpp
        "${DOCKPIPE_LAUNCHER_SOURCE_DIR}/dockpipe-launcher.qrc")
    add_executable(workspace-graph-test tests/WorkspaceGraphTest.cpp "${DOCKPIPE_LAUNCHER_SOURCE_DIR}/dockpipe-launcher.qrc")
    add_executable(spatial-workspace-test tests/SpatialWorkspaceTest.cpp)
    add_executable(desktop-app-window-test tests/DesktopAppWindowTest.cpp)
    add_executable(spatial-gpu-test tests/SpatialGpuRendererTest.cpp)
    foreach(test_target cockpit-test workspace-graph-test spatial-workspace-test desktop-app-window-test spatial-gpu-test)
        target_link_libraries(${test_target} PRIVATE dockpipe-cockpit dockpipe-launcher-ui Qt6::OpenGLWidgets)
        set_target_properties(${test_target} PROPERTIES RUNTIME_OUTPUT_DIRECTORY "${CMAKE_BINARY_DIR}")
    endforeach()
    add_test(NAME cockpit COMMAND cockpit-test)
    add_test(NAME git-review COMMAND cockpit-test --git-review)
    set_tests_properties(git-review PROPERTIES
        ENVIRONMENT "QT_QPA_PLATFORM=offscreen;DOCKPIPE_BIN=${DOCKPIPE_LAUNCHER_SOURCE_DIR}/../../../bin/dockpipe"
        TIMEOUT 120 SKIP_RETURN_CODE 77)
    add_test(NAME site-world COMMAND cockpit-test --site-world)
    add_test(NAME furnishing COMMAND cockpit-test --furnishing)
    add_test(NAME input-controls COMMAND cockpit-test --input-controls)
    add_test(NAME flight-dynamics COMMAND cockpit-test --flight-dynamics)
    add_test(NAME spatial-operations COMMAND cockpit-test --operations)
    add_test(NAME spatial-cockpit COMMAND cockpit-test --spatial)
    add_test(NAME spatial-intelligence COMMAND cockpit-test --spatial-intelligence)
    add_test(NAME spatial-civic COMMAND cockpit-test --spatial-civic)
    add_test(NAME spatial-web-model COMMAND cockpit-test --spatial-web-model)
    add_test(NAME spatial-web-browser COMMAND cockpit-test --spatial-web)
    add_test(NAME spatial-web-images COMMAND cockpit-test --spatial-web)
    set_tests_properties(spatial-web-images PROPERTIES
        ENVIRONMENT "QT_QPA_PLATFORM=offscreen;DOCKPIPE_WEB_IMAGE_FIXTURE=1;DOCKPIPE_WORKSPACE_PACKAGE=${CMAKE_CURRENT_BINARY_DIR}/assets;DOCKPIPE_COCKPIT_READER=$<TARGET_FILE:cockpit-reader>"
        TIMEOUT 45 SKIP_RETURN_CODE 77)
    add_test(NAME return-portals COMMAND cockpit-test --return-portals)
    add_test(NAME spatial-terrain COMMAND cockpit-test --spatial-terrain)
    add_test(NAME dirt-bike COMMAND cockpit-test --dirt-bike)
    add_test(NAME spatial-vehicles COMMAND cockpit-test --spatial-vehicles)
    add_test(NAME native-handoff COMMAND cockpit-test --native-handoff)
    add_test(NAME workspace-graph COMMAND workspace-graph-test)
    add_test(NAME spatial-workspace COMMAND spatial-workspace-test)
    add_test(NAME desktop-app-window COMMAND desktop-app-window-test)
    add_test(NAME spatial-gpu COMMAND spatial-gpu-test)
    set_tests_properties(spatial-gpu PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 30 SKIP_RETURN_CODE 77)
    set_tests_properties(cockpit site-world furnishing input-controls flight-dynamics spatial-operations spatial-cockpit spatial-civic spatial-intelligence spatial-web-model spatial-web-browser spatial-vehicles dirt-bike spatial-terrain return-portals
        native-handoff workspace-graph spatial-workspace desktop-app-window PROPERTIES
        ENVIRONMENT "QT_QPA_PLATFORM=offscreen;SDL_AUDIODRIVER=dummy;DOCKPIPE_WORKSPACE_PACKAGE=${CMAKE_CURRENT_BINARY_DIR}/assets;DOCKPIPE_COCKPIT_READER=$<TARGET_FILE:cockpit-reader>"
        TIMEOUT 90 SKIP_RETURN_CODE 77)
    if(TARGET remote-launcher-test)
        target_compile_definitions(remote-launcher-test PRIVATE DOCKPIPE_TEST_COCKPIT)
        target_link_libraries(remote-launcher-test PRIVATE dockpipe-cockpit)
    endif()
endif()
