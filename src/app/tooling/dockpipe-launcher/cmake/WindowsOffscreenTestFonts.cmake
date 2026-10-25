# Native Windows offscreen tests use the installed OS font collection.
# The explicit argument keeps configure tests independent of OS process discovery.
function(dockpipe_configure_windows_offscreen_test_fonts font_directory)
    if(NOT (BUILD_TESTING AND WIN32 AND CMAKE_HOST_WIN32))
        return()
    endif()

    get_property(dockpipe_test_names DIRECTORY PROPERTY TESTS)
    foreach(dockpipe_test_name IN LISTS dockpipe_test_names)
        get_test_property("${dockpipe_test_name}" ENVIRONMENT dockpipe_test_environment)
        if("QT_QPA_PLATFORM=offscreen" IN_LIST dockpipe_test_environment)
            set(dockpipe_existing_font_environment "${dockpipe_test_environment}")
            list(FILTER dockpipe_existing_font_environment INCLUDE REGEX "^QT_QPA_FONTDIR=")
            if(NOT dockpipe_existing_font_environment)
                file(TO_CMAKE_PATH "${font_directory}" dockpipe_windows_test_fonts)
                if(NOT IS_ABSOLUTE "${dockpipe_windows_test_fonts}" OR NOT IS_DIRECTORY "${dockpipe_windows_test_fonts}")
                    message(FATAL_ERROR "Windows offscreen tests require the installed OS Fonts directory")
                endif()
                set_property(TEST "${dockpipe_test_name}" APPEND PROPERTY ENVIRONMENT
                    "QT_QPA_FONTDIR=${dockpipe_windows_test_fonts}")
            endif()
        endif()
    endforeach()
endfunction()
