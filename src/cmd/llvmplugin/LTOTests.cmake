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
add_full_lto_test(full-lto-weak-resolution WEAK weak weak-other)
add_full_lto_test(full-lto-weak-resolution-reversed OTHER weak-other weak)
add_lto_error_test(full-lto-invalid-pipeline full-main BAD-PIPELINE "not-a-real-pass" full-main)
add_full_lto_test(full-lto-resolution RESOLVE weak strong)
add_full_lto_test(full-lto-resolution-reversed RESOLVE strong weak)
add_lto_error_test(full-lto-duplicate-symbol duplicate-symbol ERROR "lto<O2>" caller duplicate-symbol)

# Exercise the real optimized bitcode through the downstream GoObj producer.
# This stage intentionally stops LTO before codegen; llc supplies that next step.
set(LTO_CODEGEN_MAIN "${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/codegen-main.ll")
foreach(ARCH IN ITEMS x86_64 aarch64)
  set(LTO_CODEGEN_OUT "${CMAKE_CURRENT_BINARY_DIR}/full-lto-codegen-${ARCH}")
  file(MAKE_DIRECTORY "${LTO_CODEGEN_OUT}")
  foreach(PART IN ITEMS main dep)
    file(READ "${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/codegen-${PART}.ll" INPUT)
    if(ARCH STREQUAL "aarch64")
      string(REPLACE "x86_64" "aarch64" INPUT "${INPUT}")
      string(REPLACE "amd64" "arm64" INPUT "${INPUT}")
      string(REPLACE "GOAMD64" "GOARM64" INPUT "${INPUT}")
      string(REPLACE "v1" "v8.0" INPUT "${INPUT}")
      string(REPLACE "i64:64-i128:128-n8:16:32:64-S128"
        "i8:8:32-i16:16:32-i64:64-i128:128-v128:64:64-n32:64-S128-Fn32"
        INPUT "${INPUT}")
    endif()
    file(WRITE "${LTO_CODEGEN_OUT}/${PART}.ll" "${INPUT}")
    set_property(DIRECTORY APPEND PROPERTY CMAKE_CONFIGURE_DEPENDS
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/codegen-${PART}.ll")
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
