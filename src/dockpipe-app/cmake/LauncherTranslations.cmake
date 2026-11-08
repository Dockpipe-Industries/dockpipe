find_package(Qt6 REQUIRED COMPONENTS LinguistTools)

set(DOCKPIPE_UI_LANGUAGES en es fr de pt it nl pl ru uk tr ar hi ja ko zh_CN zh_TW)
set(dockpipe_translation_sources)
foreach(language IN LISTS DOCKPIPE_UI_LANGUAGES)
    list(APPEND dockpipe_translation_sources
        "${CMAKE_CURRENT_SOURCE_DIR}/translations/dockpipe_${language}.ts")
endforeach()

add_custom_target(dockpipe-translations-update
    COMMAND Qt6::lupdate "${CMAKE_CURRENT_SOURCE_DIR}/src"
        -extensions cpp,h,mm -no-obsolete -locations relative
        -ts ${dockpipe_translation_sources}
    COMMENT "Updating Qt Linguist catalogs for every supported platform"
    VERBATIM)

qt_add_lrelease(
    TS_FILES ${dockpipe_translation_sources}
    LRELEASE_TARGET dockpipe-translations-release
    QM_FILES_OUTPUT_VARIABLE dockpipe_translation_binaries
    OPTIONS -nounfinished)
qt_add_resources(dockpipe-launcher-ui "dockpipe_application_translations"
    PREFIX "/i18n" BASE "${CMAKE_CURRENT_BINARY_DIR}"
    FILES ${dockpipe_translation_binaries})
add_dependencies(dockpipe-launcher-ui dockpipe-translations-release)
qt_add_resources(dockpipe-workspace-reader "dockpipe_reader_translations"
    PREFIX "/i18n" BASE "${CMAKE_CURRENT_BINARY_DIR}"
    FILES ${dockpipe_translation_binaries})
add_dependencies(dockpipe-workspace-reader dockpipe-translations-release)

# Bundle Qt's matching standard-dialog translations with the SDK UI library.
# Optional catalogs are absent in some Qt distributions; application catalogs
# remain required and embedded for every language advertised by the selector.
get_target_property(qt_qmake_executable Qt6::qmake IMPORTED_LOCATION)
execute_process(COMMAND "${qt_qmake_executable}" -query QT_INSTALL_TRANSLATIONS
    OUTPUT_VARIABLE qt_translation_directory OUTPUT_STRIP_TRAILING_WHITESPACE
    RESULT_VARIABLE qt_translation_query_result)
if(qt_translation_query_result EQUAL 0)
    set(qt_framework_catalogs)
    foreach(language IN LISTS DOCKPIPE_UI_LANGUAGES)
        set(catalog "${qt_translation_directory}/qtbase_${language}.qm")
        if(language STREQUAL "pt" AND NOT EXISTS "${catalog}")
            set(catalog "${qt_translation_directory}/qtbase_pt_BR.qm")
        endif()
        if(EXISTS "${catalog}")
            set_source_files_properties("${catalog}" PROPERTIES QT_RESOURCE_ALIAS "qtbase_${language}.qm")
            list(APPEND qt_framework_catalogs "${catalog}")
        endif()
    endforeach()
    if(qt_framework_catalogs)
        qt_add_resources(dockpipe-launcher-ui "dockpipe_qt_translations"
            PREFIX "/i18n" FILES ${qt_framework_catalogs})
    endif()
endif()

# Qt does not ship a Hindi qtbase catalog. Supply its standard button vocabulary
# without presenting it as a complete translation of the external Qt framework.
if(NOT EXISTS "${qt_translation_directory}/qtbase_hi.qm")
    qt_add_lrelease(TS_FILES "${CMAKE_CURRENT_SOURCE_DIR}/translations/qtbase_hi.ts"
        LRELEASE_TARGET dockpipe-hindi-buttons-release
        QM_FILES_OUTPUT_VARIABLE dockpipe_hindi_buttons)
    qt_add_resources(dockpipe-launcher-ui "dockpipe_hindi_buttons"
        PREFIX "/i18n" BASE "${CMAKE_CURRENT_BINARY_DIR}" FILES ${dockpipe_hindi_buttons})
    add_dependencies(dockpipe-launcher-ui dockpipe-hindi-buttons-release)
endif()

if(BUILD_TESTING)
    add_executable(launcher-translations-test tests/LauncherTranslationsTest.cpp)
    target_link_libraries(launcher-translations-test PRIVATE dockpipe-launcher-ui Qt6::Test)
    add_test(NAME launcher-translations COMMAND launcher-translations-test)
    set_tests_properties(launcher-translations PROPERTIES
        ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 30)
    if(Python3_Interpreter_FOUND)
        add_test(NAME translation-catalogs
            COMMAND "${Python3_EXECUTABLE}" "${CMAKE_CURRENT_SOURCE_DIR}/tests/test_translations.py"
                --lupdate "$<TARGET_FILE:Qt6::lupdate>")
    endif()
endif()
