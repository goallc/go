# Only a test harness, never installed as a toolchain executable.
add_executable(goallc-lto-test testdata/lto/driver.cpp)
llvm_update_compile_flags(goallc-lto-test)
target_include_directories(goallc-lto-test PRIVATE ${CMAKE_CURRENT_SOURCE_DIR})
target_compile_features(goallc-lto-test PRIVATE cxx_std_17)

add_test(NAME full-lto-single-object
  COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
    "'$<TARGET_FILE:goallc-lto-test>' --output-dir='${CMAKE_CURRENT_BINARY_DIR}/full-single' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/single-main.ll' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/single-dep.ll' && '${GOALLC_FILECHECK_EXECUTABLE}' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/single-main.ll' --input-file='${CMAKE_CURRENT_BINARY_DIR}/full-single/program.ll' && '${GOALLC_FILECHECK_EXECUTABLE}' --check-prefix=MANIFEST '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/single-main.ll' --input-file='${CMAKE_CURRENT_BINARY_DIR}/full-single/program.json'")
target_link_libraries(goallc-lto-test PRIVATE GoALLCStatepointsStatic LLVM)
# FullLTO always emits program.bc/program.json; only ThinLTO emits partitions.
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
