# Only a test harness, never installed as a toolchain executable.
add_executable(goallc-lto-test testdata/lto/driver.cpp)
llvm_update_compile_flags(goallc-lto-test)
target_include_directories(goallc-lto-test PRIVATE ${CMAKE_CURRENT_SOURCE_DIR})
target_compile_features(goallc-lto-test PRIVATE cxx_std_17)

add_test(NAME full-lto-single-object
  COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
    "'$<TARGET_FILE:goallc-lto-test>' --output-dir='${CMAKE_CURRENT_BINARY_DIR}/full-single' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/single-main.ll' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/single-dep.ll' && '${GOALLC_FILECHECK_EXECUTABLE}' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/single-main.ll' --input-file='${CMAKE_CURRENT_BINARY_DIR}/full-single/program.ll' && '${GOALLC_FILECHECK_EXECUTABLE}' --check-prefix=MANIFEST '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/single-main.ll' --input-file='${CMAKE_CURRENT_BINARY_DIR}/full-single/program.json'")
target_link_libraries(goallc-lto-test PRIVATE GoALLCStatepointsStatic LLVM)
# FullLTO emits one optimized module and a package manifest.
function(add_full_lto_test NAME PREFIX)
  set(OUT "${CMAKE_CURRENT_BINARY_DIR}/${NAME}")
  set(COMMAND "'$<TARGET_FILE:goallc-lto-test>' --output-dir='${OUT}'")
  foreach(INPUT IN LISTS ARGN)
    string(APPEND COMMAND " '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/${INPUT}.ll'")
  endforeach()
  set(CHECK "${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/full-main.ll")
  string(APPEND COMMAND " '${CHECK}' && '${GOALLC_FILECHECK_EXECUTABLE}' '${CHECK}' --check-prefix='${PREFIX}' --input-file='${OUT}/program.ll'")
  if(PREFIX STREQUAL "META")
    string(APPEND COMMAND " && '${GOALLC_FILECHECK_EXECUTABLE}' '${CHECK}' --check-prefix=MANIFEST --input-file='${OUT}/program.json'")
  endif()
  if(PREFIX STREQUAL "CROSS")
    string(APPEND COMMAND " && '${GOALLC_FILECHECK_EXECUTABLE}' '${CHECK}' --check-prefix=CROSS-MANIFEST --input-file='${OUT}/program.json'")
  endif()
  add_test(NAME ${NAME} COMMAND ${GOALLC_BASH_EXECUTABLE} -o pipefail -c "${COMMAND}")
endfunction()

function(add_lto_error_test NAME CHECK PREFIX PIPELINE)
  set(COMMAND "'${GOALLC_NOT_EXECUTABLE}' '$<TARGET_FILE:goallc-lto-test>' --output-dir='${CMAKE_CURRENT_BINARY_DIR}/${NAME}' --passes='${PIPELINE}'")
  foreach(INPUT IN LISTS ARGN)
    string(APPEND COMMAND " '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/${INPUT}.ll'")
  endforeach()
  string(APPEND COMMAND " 2>&1 | '${GOALLC_FILECHECK_EXECUTABLE}' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/${CHECK}.ll' --check-prefix='${PREFIX}'")
  add_test(NAME ${NAME} COMMAND ${GOALLC_BASH_EXECUTABLE} -o pipefail -c "${COMMAND}")
endfunction()

add_full_lto_test(full-lto-cross-package CROSS caller callee)
add_full_lto_test(full-lto-package-metadata META metadata-caller metadata-callee metadata-leaf)
add_full_lto_test(full-lto-bss BSS bss-declaration bss-definition)
add_lto_error_test(full-lto-bss-too-small bss-too-small ERROR "lto<O2>" bss-declaration bss-too-small)
add_lto_error_test(full-lto-duplicate-package caller DUPLICATE "lto<O2>" caller caller)
add_lto_error_test(full-lto-incompatible-config incompatible-config ERROR "lto<O2>" caller incompatible-config)
add_lto_error_test(full-lto-duplicate-index duplicate-index ERROR "lto<O2>" duplicate-index)
add_lto_error_test(full-lto-new-definitions global-dtors ERROR lower-global-dtors global-dtors)
add_lto_error_test(full-lto-input-alias alias ERROR "lto<O2>" alias)
add_lto_error_test(full-lto-fips-boundary fips ERROR "lto<O2>" fips)

add_full_lto_test(full-lto-abi-and-closure ABI abi-closure-caller abi-closure-callee)
add_test(NAME full-lto-abi0
  COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
    "'$<TARGET_FILE:goallc-lto-test>' --output-dir='${CMAKE_CURRENT_BINARY_DIR}/full-abi0' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/abi-closure-caller.ll' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/abi-closure-callee.ll' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/full-main.ll' && '${GOALLC_FILECHECK_EXECUTABLE}' --check-prefix=ABI0 '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/full-main.ll' --input-file='${CMAKE_CURRENT_BINARY_DIR}/full-abi0/program.ll'")

add_full_lto_test(full-lto-bss-reversed BSS bss-definition bss-declaration)
add_full_lto_test(full-lto-common COMMON bss-small bss-large)
add_full_lto_test(full-lto-common-reversed COMMON bss-large bss-small)
add_full_lto_test(full-lto-common-initialized INIT bss-small bss-initialized)
add_full_lto_test(full-lto-common-initialized-reversed INIT bss-initialized bss-small)
add_full_lto_test(full-lto-linkname-function LINKNAME linkname-placeholder linkname-function)
add_full_lto_test(full-lto-linkname-function-reversed LINKNAME linkname-function linkname-placeholder)
add_full_lto_test(full-lto-weak-resolution WEAK weak weak-other)
add_full_lto_test(full-lto-weak-resolution-reversed OTHER weak-other weak)
add_lto_error_test(full-lto-invalid-pipeline full-main BAD-PIPELINE "not-a-real-pass" full-main)
add_full_lto_test(full-lto-resolution RESOLVE weak strong)
add_full_lto_test(full-lto-resolution-reversed RESOLVE strong weak)
add_lto_error_test(full-lto-duplicate-symbol duplicate-symbol ERROR "lto<O2>" caller duplicate-symbol)

function(configure_lto_arch SOURCE OUTPUT ARCH)
  file(READ "${SOURCE}" INPUT)
  if(ARCH STREQUAL "aarch64")
    string(REPLACE "x86_64" "aarch64" INPUT "${INPUT}")
    string(REPLACE "amd64" "arm64" INPUT "${INPUT}")
    string(REPLACE "GOAMD64" "GOARM64" INPUT "${INPUT}")
    string(REPLACE "v1" "v8.0" INPUT "${INPUT}")
    string(REPLACE "i64:64-i128:128-n8:16:32:64-S128"
      "i8:8:32-i16:16:32-i64:64-i128:128-v128:64:64-n32:64-S128-Fn32"
      INPUT "${INPUT}")
  endif()
  file(WRITE "${OUTPUT}" "${INPUT}")
  set_property(DIRECTORY APPEND PROPERTY CMAKE_CONFIGURE_DEPENDS "${SOURCE}")
endfunction()

# Exercise the real optimized bitcode through the downstream GoObj producer.
# This stage intentionally stops LTO before codegen; llc supplies that next step.
set(LTO_CODEGEN_MAIN "${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/codegen-main.ll")
foreach(ARCH IN ITEMS x86_64 aarch64)
  set(LTO_CODEGEN_OUT "${CMAKE_CURRENT_BINARY_DIR}/full-lto-codegen-${ARCH}")
  file(MAKE_DIRECTORY "${LTO_CODEGEN_OUT}")
  foreach(PART IN ITEMS main dep)
    configure_lto_arch("${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/codegen-${PART}.ll"
      "${LTO_CODEGEN_OUT}/${PART}.ll" "${ARCH}")
  endforeach()
  add_test(NAME full-lto-codegen-ir-${ARCH}
    COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
      "'$<TARGET_FILE:goallc-lto-test>' --output-dir='${LTO_CODEGEN_OUT}' '${LTO_CODEGEN_OUT}/main.ll' '${LTO_CODEGEN_OUT}/dep.ll' && '${GOALLC_FILECHECK_EXECUTABLE}' --check-prefix=IR '${LTO_CODEGEN_MAIN}' --input-file='${LTO_CODEGEN_OUT}/program.ll'")
  add_test(NAME full-lto-codegen-${ARCH}
    COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
      "'${GOALLC_LLC_EXECUTABLE}' '-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>' -goallc-pass-plugin-report -frame-pointer=all -verify-machineinstrs -filetype=obj '${LTO_CODEGEN_OUT}/program.bc' -o '${LTO_CODEGEN_OUT}/program.goobj' 2>&1 | '${GOALLC_FILECHECK_EXECUTABLE}' --check-prefix=CALLBACK '${LTO_CODEGEN_MAIN}'")
  set_tests_properties(full-lto-codegen-${ARCH} PROPERTIES DEPENDS full-lto-codegen-ir-${ARCH})
  if(GOALLC_OBJVIEW_EXECUTABLE)
    add_test(NAME full-lto-objview-${ARCH}
      COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
        "'${GOALLC_OBJVIEW_EXECUTABLE}' -format=text '${LTO_CODEGEN_OUT}/program.goobj' | '${GOALLC_FILECHECK_EXECUTABLE}' --check-prefix=OBJ '${LTO_CODEGEN_MAIN}'")
    set_tests_properties(full-lto-objview-${ARCH} PROPERTIES DEPENDS full-lto-codegen-${ARCH})
  endif()
endforeach()

add_test(NAME full-lto-no-save-ir
  COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
    "'${CMAKE_COMMAND}' -E rm -f '${CMAKE_CURRENT_BINARY_DIR}/full-no-save/program.ll' && '$<TARGET_FILE:goallc-lto-test>' --save-ir=false --output-dir='${CMAKE_CURRENT_BINARY_DIR}/full-no-save' '${LTO_CODEGEN_MAIN}' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/codegen-dep.ll' && test ! -e '${CMAKE_CURRENT_BINARY_DIR}/full-no-save/program.ll' && test -s '${CMAKE_CURRENT_BINARY_DIR}/full-no-save/program.json' && '${GOALLC_OPT_EXECUTABLE}' -passes=verify -disable-output '${CMAKE_CURRENT_BINARY_DIR}/full-no-save/program.bc'")

add_lto_error_test(full-lto-bss-too-small-reversed bss-too-small ERROR "lto<O2>" bss-too-small bss-declaration)
add_lto_error_test(full-lto-bss-function-error ordinary-placeholder ERROR "lto<O2>" ordinary-placeholder linkname-function)
add_test(NAME full-lto-local-collision
  COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
    "'$<TARGET_FILE:goallc-lto-test>' --output-dir='${CMAKE_CURRENT_BINARY_DIR}/full-local' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/local-a.ll' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/local-b.ll' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/full-main.ll' && cat '${CMAKE_CURRENT_BINARY_DIR}/full-local/program.ll' '${CMAKE_CURRENT_BINARY_DIR}/full-local/program.json' | '${GOALLC_FILECHECK_EXECUTABLE}' --check-prefix=LOCAL '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/local-a.ll'")

if(GOALLC_OBJVIEW_EXECUTABLE)
  add_test(NAME full-lto-common-goobj
    COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
      "'${GOALLC_LLC_EXECUTABLE}' '-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>' -verify-machineinstrs -filetype=obj '${CMAKE_CURRENT_BINARY_DIR}/full-lto-common/program.bc' -o '${CMAKE_CURRENT_BINARY_DIR}/full-lto-common/program.goobj' && '${GOALLC_OBJVIEW_EXECUTABLE}' -json '${CMAKE_CURRENT_BINARY_DIR}/full-lto-common/program.goobj' | '${GOALLC_FILECHECK_EXECUTABLE}' --check-prefix=OBJ '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/bss-large.ll'")
  set_tests_properties(full-lto-common-goobj PROPERTIES DEPENDS full-lto-common)
endif()

# Native DI nodes and Go relation attachments need no LTO transport tables.
foreach(ORDER IN ITEMS forward reversed)
  set(DEBUG_INPUTS "'${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/native-debug-main.ll' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/native-debug-dep.ll'")
  if(ORDER STREQUAL "reversed")
    set(DEBUG_INPUTS "'${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/native-debug-dep.ll' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/native-debug-main.ll'")
  endif()
  set(DEBUG_OUT "${CMAKE_CURRENT_BINARY_DIR}/full-lto-native-debug-${ORDER}")
  add_test(NAME full-lto-native-debug-${ORDER}
    COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
      "'$<TARGET_FILE:goallc-lto-test>' --output-dir='${DEBUG_OUT}' ${DEBUG_INPUTS} && '${GOALLC_FILECHECK_EXECUTABLE}' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/native-debug-main.ll' --check-prefix=NATIVE --implicit-check-not='!goobj.debug.config =' --implicit-check-not='!goobj.debug.vars =' --implicit-check-not='!goobj.debug.funcs =' --input-file='${DEBUG_OUT}/program.ll' && '${GOALLC_FILECHECK_EXECUTABLE}' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/native-debug-main.ll' --check-prefix=MANIFEST --input-file='${DEBUG_OUT}/program.json'")
endforeach()

# Content-addressed text keeps native cross-package hashing after a single
# codegen pass. Same-name bodies must not resolve by LLVM linkage alone.
foreach(ARCH IN ITEMS x86_64 aarch64)
  set(CONTENT_OUT "${CMAKE_CURRENT_BINARY_DIR}/full-lto-content-${ARCH}")
  set(CONTENT_CHECK "${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/content-main.ll")
  file(MAKE_DIRECTORY "${CONTENT_OUT}")
  foreach(PART IN ITEMS main dep)
    configure_lto_arch("${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/content-${PART}.ll"
      "${CONTENT_OUT}/${PART}.ll" "${ARCH}")
  endforeach()
  add_test(NAME full-lto-content-${ARCH}
    COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
      "'$<TARGET_FILE:goallc-lto-test>' --output-dir='${CONTENT_OUT}' '${CONTENT_OUT}/main.ll' '${CONTENT_OUT}/dep.ll' && '${GOALLC_FILECHECK_EXECUTABLE}' '${CONTENT_CHECK}' --check-prefix=IR --implicit-check-not='.goallc.lto.' --input-file='${CONTENT_OUT}/program.ll' && '${GOALLC_FILECHECK_EXECUTABLE}' '${CONTENT_CHECK}' --check-prefix=MANIFEST --input-file='${CONTENT_OUT}/program.json'")
  if(GOALLC_OBJVIEW_EXECUTABLE)
    add_test(NAME full-lto-content-goobj-${ARCH}
      COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
        "'${GOALLC_LLC_EXECUTABLE}' -filetype=obj '${CONTENT_OUT}/program.bc' -o '${CONTENT_OUT}/program.goobj' && '${GOALLC_OBJVIEW_EXECUTABLE}' -format=raw '${CONTENT_OUT}/program.goobj' | '${GOALLC_FILECHECK_EXECUTABLE}' '${CONTENT_CHECK}' --check-prefix=OBJ --implicit-check-not='dep.Closure.1'")
    set_tests_properties(full-lto-content-goobj-${ARCH} PROPERTIES DEPENDS full-lto-content-${ARCH})
  endif()
endforeach()

add_lto_error_test(full-lto-unprepared-input unprepared ERROR "lto<O2>" unprepared)
