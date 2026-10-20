if(NOT IS_ABSOLUTE "${DOCKPIPE_COCKPIT_ASSETS}" OR NOT IS_DIRECTORY "${DOCKPIPE_COCKPIT_ASSETS}")
    message(FATAL_ERROR "DOCKPIPE_COCKPIT_ASSETS must name an absolute installed assets directory")
endif()
if(NOT CMAKE_SYSTEM_NAME STREQUAL "Linux")
    message(FATAL_ERROR "Installed Cockpit rendering qualification currently supports Linux only")
endif()
# New packages use the host-owned reader. Historical 0.1.2 fixtures retain
# their admitted helper name; this does not select a package or alter its lock.
set(_dockpipe_cockpit_reader "${DOCKPIPE_COCKPIT_ASSETS}/ui/dockpipe-workspace-reader")
if(NOT EXISTS "${_dockpipe_cockpit_reader}")
    set(_dockpipe_cockpit_reader "${DOCKPIPE_COCKPIT_ASSETS}/ui/cockpit-reader")
endif()
if(NOT EXISTS "${_dockpipe_cockpit_reader}")
    message(FATAL_ERROR "Installed Cockpit assets are missing the admitted filesystem reader")
endif()
foreach(payload IN ITEMS ui/libdockpipe-cockpit.so)
    if(NOT EXISTS "${DOCKPIPE_COCKPIT_ASSETS}/${payload}")
        message(FATAL_ERROR "Installed Cockpit assets are missing ${payload}")
    endif()
endforeach()

find_package(Qt6 6.10.3 EXACT REQUIRED COMPONENTS OpenGLWidgets)
find_program(DOCKPIPE_XVFB_RUN NAMES xvfb-run REQUIRED)

add_executable(cockpit-package-rendering-test tests/PackageRenderingTest.cpp)
target_link_libraries(cockpit-package-rendering-test PRIVATE dockpipe-launcher-ui)
add_library(cockpit-rendering-viewport-fixture SHARED tests/RenderingViewportFixture.cpp)
target_link_libraries(cockpit-rendering-viewport-fixture PRIVATE Qt6::OpenGLWidgets)
target_compile_definitions(cockpit-package-rendering-test PRIVATE
    DOCKPIPE_TEST_GL_FACTORY_PATH="$<TARGET_FILE:cockpit-rendering-viewport-fixture>")
add_dependencies(cockpit-package-rendering-test cockpit-rendering-viewport-fixture)

# A skip from this required real X11 context fails qualification. Offscreen
# rendering cannot establish the late-import compositor contract.
add_test(NAME cockpit-package-rendering
    COMMAND "${DOCKPIPE_XVFB_RUN}" -a "${CMAKE_COMMAND}" -E env
        QT_QPA_PLATFORM=xcb LIBGL_ALWAYS_SOFTWARE=1 SDL_AUDIODRIVER=dummy
        "DOCKPIPE_WORKSPACE_PACKAGE=${DOCKPIPE_COCKPIT_ASSETS}"
        "DOCKPIPE_WORKSPACE_READER=${_dockpipe_cockpit_reader}"
        "DOCKPIPE_COCKPIT_READER=${_dockpipe_cockpit_reader}"
        "$<TARGET_FILE:cockpit-package-rendering-test>")
set_tests_properties(cockpit-package-rendering PROPERTIES TIMEOUT 90)
