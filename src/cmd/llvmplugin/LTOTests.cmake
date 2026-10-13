# Only a test harness, never installed as a toolchain executable.
add_executable(goallc-lto-test testdata/lto/driver.cpp)
llvm_update_compile_flags(goallc-lto-test)
target_include_directories(goallc-lto-test PRIVATE ${CMAKE_CURRENT_SOURCE_DIR})
target_compile_features(goallc-lto-test PRIVATE cxx_std_17)

add_test(NAME full-lto-single-object
  COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
    "'$<TARGET_FILE:goallc-lto-test>' --single --output-dir='${CMAKE_CURRENT_BINARY_DIR}/full-single' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/single-main.ll' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/single-dep.ll' && '${GOALLC_FILECHECK_EXECUTABLE}' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/single-main.ll' --input-file='${CMAKE_CURRENT_BINARY_DIR}/full-single/program.ll' && '${GOALLC_FILECHECK_EXECUTABLE}' --check-prefix=MANIFEST '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/single-main.ll' --input-file='${CMAKE_CURRENT_BINARY_DIR}/full-single/program.json'")
target_link_libraries(goallc-lto-test PRIVATE GoALLCStatepointsStatic LLVM)
add_test(NAME full-lto-cross-package
  COMMAND ${GOALLC_BASH_EXECUTABLE} -o pipefail -c
    "'$<TARGET_FILE:goallc-lto-test>' --output-dir='${CMAKE_CURRENT_BINARY_DIR}/test-output' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/caller.ll' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/callee.ll' && '${GOALLC_FILECHECK_EXECUTABLE}' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/caller.ll' --input-file='${CMAKE_CURRENT_BINARY_DIR}/test-output/0.ll' && '${GOALLC_FILECHECK_EXECUTABLE}' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/callee.ll' --input-file='${CMAKE_CURRENT_BINARY_DIR}/test-output/1.ll'")

function(add_lto_test NAME)
  set(OUT "${CMAKE_CURRENT_BINARY_DIR}/${NAME}")
  set(COMMAND "'$<TARGET_FILE:goallc-lto-test>' --output-dir='${OUT}'")
  set(CHECKS "")
  set(INDEX 0)
  foreach(INPUT IN LISTS ARGN)
    set(SOURCE "${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/${INPUT}.ll")
    string(APPEND COMMAND " '${SOURCE}'")
    string(APPEND CHECKS " && '${GOALLC_FILECHECK_EXECUTABLE}' '${SOURCE}' --input-file='${OUT}/${INDEX}.ll'")
    math(EXPR INDEX "${INDEX} + 1")
  endforeach()
  add_test(NAME ${NAME} COMMAND ${GOALLC_BASH_EXECUTABLE} -o pipefail -c "${COMMAND}${CHECKS}")
endfunction()

function(add_lto_error_test NAME CHECK PREFIX PIPELINE)
  set(COMMAND "'${GOALLC_NOT_EXECUTABLE}' '$<TARGET_FILE:goallc-lto-test>' --output-dir='${CMAKE_CURRENT_BINARY_DIR}/${NAME}' --passes='${PIPELINE}'")
  foreach(INPUT IN LISTS ARGN)
    string(APPEND COMMAND " '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/${INPUT}.ll'")
  endforeach()
  string(APPEND COMMAND " 2>&1 | '${GOALLC_FILECHECK_EXECUTABLE}' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/${CHECK}.ll' --check-prefix='${PREFIX}'")
  add_test(NAME ${NAME} COMMAND ${GOALLC_BASH_EXECUTABLE} -o pipefail -c "${COMMAND}")
endfunction()

add_lto_test(full-lto-package-metadata metadata-caller metadata-callee metadata-leaf)
add_lto_test(full-lto-bss bss-declaration bss-definition)
add_lto_error_test(full-lto-bss-too-small bss-too-small ERROR "lto<O2>" bss-declaration bss-too-small)
add_lto_error_test(full-lto-duplicate-package caller DUPLICATE "lto<O2>" caller caller)
add_lto_error_test(full-lto-incompatible-config incompatible-config ERROR "lto<O2>" caller incompatible-config)
add_lto_error_test(full-lto-duplicate-index duplicate-index ERROR "lto<O2>" duplicate-index)
add_lto_error_test(full-lto-new-definitions global-dtors ERROR lower-global-dtors global-dtors)
add_lto_error_test(full-lto-input-alias alias ERROR "lto<O2>" alias)

add_test(NAME thin-lto-cross-package
  COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
    "'$<TARGET_FILE:goallc-lto-test>' --thin --output-dir='${CMAKE_CURRENT_BINARY_DIR}/thin-cross' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/caller.ll' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/callee.ll' && '${GOALLC_FILECHECK_EXECUTABLE}' --check-prefix=THIN '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/caller.ll' --input-file='${CMAKE_CURRENT_BINARY_DIR}/thin-cross/0.ll' && '${GOALLC_FILECHECK_EXECUTABLE}' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/callee.ll' --input-file='${CMAKE_CURRENT_BINARY_DIR}/thin-cross/1.ll'")
add_test(NAME thin-lto-package-metadata
  COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
    "'$<TARGET_FILE:goallc-lto-test>' --thin --output-dir='${CMAKE_CURRENT_BINARY_DIR}/thin-metadata' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/metadata-caller.ll' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/metadata-callee.ll' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/metadata-leaf.ll' && '${GOALLC_FILECHECK_EXECUTABLE}' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/metadata-caller.ll' --input-file='${CMAKE_CURRENT_BINARY_DIR}/thin-metadata/0.ll' && '${GOALLC_FILECHECK_EXECUTABLE}' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/metadata-callee.ll' --input-file='${CMAKE_CURRENT_BINARY_DIR}/thin-metadata/1.ll' && '${GOALLC_FILECHECK_EXECUTABLE}' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/metadata-leaf.ll' --input-file='${CMAKE_CURRENT_BINARY_DIR}/thin-metadata/2.ll'")

add_test(NAME thin-lto-local-name-collision
  COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
    "'$<TARGET_FILE:goallc-lto-test>' --thin --output-dir='${CMAKE_CURRENT_BINARY_DIR}/thin-local' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/thin-local-caller.ll' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/thin-local-callee.ll' && '${GOALLC_FILECHECK_EXECUTABLE}' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/thin-local-caller.ll' --input-file='${CMAKE_CURRENT_BINARY_DIR}/thin-local/0.ll' && '${GOALLC_FILECHECK_EXECUTABLE}' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/thin-local-callee.ll' --input-file='${CMAKE_CURRENT_BINARY_DIR}/thin-local/1.ll'")

add_test(NAME full-lto-abi-and-closure
  COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
    "'$<TARGET_FILE:goallc-lto-test>'  --output-dir='${CMAKE_CURRENT_BINARY_DIR}/full-abi' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/abi-closure-caller.ll' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/abi-closure-callee.ll' && '${GOALLC_FILECHECK_EXECUTABLE}' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/abi-closure-caller.ll' --input-file='${CMAKE_CURRENT_BINARY_DIR}/full-abi/0.ll' && '${GOALLC_FILECHECK_EXECUTABLE}' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/abi-closure-callee.ll' --input-file='${CMAKE_CURRENT_BINARY_DIR}/full-abi/1.ll' && '${GOALLC_FILECHECK_EXECUTABLE}' --check-prefix=ABI0 '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/abi-closure-caller.ll' --input-file='${CMAKE_CURRENT_BINARY_DIR}/full-abi/0.ll' && '${GOALLC_FILECHECK_EXECUTABLE}' --check-prefix=ABI0 '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/abi-closure-callee.ll' --input-file='${CMAKE_CURRENT_BINARY_DIR}/full-abi/1.ll'")
add_test(NAME thin-lto-abi-and-closure
  COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
    "'$<TARGET_FILE:goallc-lto-test>' --thin --output-dir='${CMAKE_CURRENT_BINARY_DIR}/thin-abi' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/abi-closure-caller.ll' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/abi-closure-callee.ll' && '${GOALLC_FILECHECK_EXECUTABLE}' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/abi-closure-caller.ll' --input-file='${CMAKE_CURRENT_BINARY_DIR}/thin-abi/0.ll' && '${GOALLC_FILECHECK_EXECUTABLE}' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/abi-closure-callee.ll' --input-file='${CMAKE_CURRENT_BINARY_DIR}/thin-abi/1.ll' && '${GOALLC_FILECHECK_EXECUTABLE}' --check-prefix=ABI0 '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/abi-closure-caller.ll' --input-file='${CMAKE_CURRENT_BINARY_DIR}/thin-abi/0.ll' && '${GOALLC_FILECHECK_EXECUTABLE}' --check-prefix=ABI0 '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/abi-closure-callee.ll' --input-file='${CMAKE_CURRENT_BINARY_DIR}/thin-abi/1.ll'")

add_test(NAME thin-lto-content-and-metadata
  COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
    "'$<TARGET_FILE:goallc-lto-test>' --thin --output-dir='${CMAKE_CURRENT_BINARY_DIR}/thin-content' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/content-caller.ll' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/content-callee.ll' && '${GOALLC_FILECHECK_EXECUTABLE}' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/content-caller.ll' --input-file='${CMAKE_CURRENT_BINARY_DIR}/thin-content/0.ll' && '${GOALLC_FILECHECK_EXECUTABLE}' '${CMAKE_CURRENT_SOURCE_DIR}/testdata/lto/content-callee.ll' --input-file='${CMAKE_CURRENT_BINARY_DIR}/thin-content/1.ll'")
