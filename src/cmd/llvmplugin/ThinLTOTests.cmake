add_executable(goallc-thinlto-test testdata/thinlto/driver.cpp)
llvm_update_compile_flags(goallc-thinlto-test)
target_include_directories(goallc-thinlto-test PRIVATE ${CMAKE_CURRENT_SOURCE_DIR})
target_compile_features(goallc-thinlto-test PRIVATE cxx_std_17)
target_link_libraries(goallc-thinlto-test PRIVATE GoALLCStatepointsStatic LLVM)
set(THIN_FIXTURES "${CMAKE_CURRENT_SOURCE_DIR}/testdata/thinlto")
set(THIN_OUT "${CMAKE_CURRENT_BINARY_DIR}/thinlto-cross-package")
add_test(NAME thinlto-cross-package
  COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
  "'$<TARGET_FILE:goallc-thinlto-test>' --output-dir='${THIN_OUT}' '${THIN_FIXTURES}/caller.ll' '${THIN_FIXTURES}/callee.ll' && '${GOALLC_FILECHECK_EXECUTABLE}' --implicit-check-not='define available_externally' '${THIN_FIXTURES}/caller.ll' --input-file='${THIN_OUT}/0.ll' && '${GOALLC_FILECHECK_EXECUTABLE}' '${THIN_FIXTURES}/callee.ll' --input-file='${THIN_OUT}/1.ll'")

set(THIN_DEBUG "${CMAKE_CURRENT_BINARY_DIR}/thinlto-debug")
add_test(NAME thinlto-debug
  COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
  "'$<TARGET_FILE:goallc-thinlto-test>' --output-dir='${THIN_DEBUG}' '${THIN_FIXTURES}/debug-caller.ll' '${THIN_FIXTURES}/debug-callee.ll' && '${GOALLC_FILECHECK_EXECUTABLE}' '${THIN_FIXTURES}/debug-caller.ll' --input-file='${THIN_DEBUG}/0.ll'")
set_tests_properties(thinlto-debug PROPERTIES FIXTURES_SETUP thinlto-debug-ir)

set(THIN_FIPS "${CMAKE_CURRENT_BINARY_DIR}/thinlto-fips")
add_test(NAME thinlto-fips
  COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
  "'$<TARGET_FILE:goallc-thinlto-test>' --output-dir='${THIN_FIPS}' '${THIN_FIXTURES}/fips-caller.ll' '${THIN_FIXTURES}/fips-callee.ll' && '${GOALLC_FILECHECK_EXECUTABLE}' --implicit-check-not='!goobj.static_rodata_type' '${THIN_FIXTURES}/fips-caller.ll' --input-file='${THIN_FIPS}/0.ll' && '${GOALLC_FILECHECK_EXECUTABLE}' '${THIN_FIXTURES}/fips-callee.ll' --input-file='${THIN_FIPS}/1.ll'")
set_tests_properties(thinlto-fips PROPERTIES FIXTURES_SETUP thinlto-fips-ir)
foreach(ARCH IN ITEMS x86_64 aarch64)
  foreach(INDEX RANGE 0 1)
    if(INDEX EQUAL 0)
      set(FIXTURE fips-caller.ll)
    else()
      set(FIXTURE fips-callee.ll)
    endif()
    add_test(NAME thinlto-fips-${ARCH}-${INDEX}
      COMMAND "${GOALLC_LLC_EXECUTABLE}"
        "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
        "-mtriple=${ARCH}-unknown-linux-goobj"
        -frame-pointer=all -verify-machineinstrs -filetype=obj
        -o "${THIN_FIPS}/${ARCH}-${INDEX}.o" "${THIN_FIPS}/${INDEX}.bc")
    set_tests_properties(thinlto-fips-${ARCH}-${INDEX} PROPERTIES
      FIXTURES_REQUIRED thinlto-fips-ir FIXTURES_SETUP thinlto-fips-${ARCH}-${INDEX}-obj)
    if(GOALLC_OBJVIEW_EXECUTABLE)
      add_test(NAME thinlto-fips-${ARCH}-${INDEX}-object
        COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
        "'${GOALLC_OBJVIEW_EXECUTABLE}' -json '${THIN_FIPS}/${ARCH}-${INDEX}.o' | '${GOALLC_FILECHECK_EXECUTABLE}' --check-prefix=OBJ '${THIN_FIXTURES}/${FIXTURE}'")
      set_tests_properties(thinlto-fips-${ARCH}-${INDEX}-object PROPERTIES
        FIXTURES_REQUIRED thinlto-fips-${ARCH}-${INDEX}-obj)
    endif()
  endforeach()
endforeach()

foreach(ARCH IN ITEMS x86_64 aarch64)
  add_test(NAME thinlto-debug-${ARCH}
    COMMAND "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      "-mtriple=${ARCH}-unknown-linux-goobj"
      -frame-pointer=all -verify-machineinstrs -filetype=obj
      -o "${THIN_DEBUG}/${ARCH}.o" "${THIN_DEBUG}/0.bc")
  set_tests_properties(thinlto-debug-${ARCH} PROPERTIES
    FIXTURES_REQUIRED thinlto-debug-ir FIXTURES_SETUP thinlto-debug-${ARCH}-obj)
  if(GOALLC_OBJVIEW_EXECUTABLE)
    add_test(NAME thinlto-debug-${ARCH}-object
      COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
      "'${GOALLC_OBJVIEW_EXECUTABLE}' -json '${THIN_DEBUG}/${ARCH}.o' | '${GOALLC_FILECHECK_EXECUTABLE}' --check-prefix=OBJ '${THIN_FIXTURES}/debug-caller.ll'")
    set_tests_properties(thinlto-debug-${ARCH}-object PROPERTIES
      FIXTURES_REQUIRED thinlto-debug-${ARCH}-obj)
  endif()
endforeach()

# Exercise the production buffer ABI with synthetic module identifiers:
# inputs and optimized modules must never be reopened from disk. No diagnostic
# directory may be created when save-temps is disabled.
add_test(NAME thinlto-memory-objects
  COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
  "rm -rf '${THIN_OUT}/no-files' && '$<TARGET_FILE:goallc-thinlto-test>' --emit-objects --save-temps=false --output-dir='${THIN_OUT}/no-files' '${THIN_FIXTURES}/debug-caller.ll' '${THIN_FIXTURES}/debug-callee.ll' | '${GOALLC_FILECHECK_EXECUTABLE}' --check-prefix=BUFFER '${THIN_FIXTURES}/debug-caller.ll'")
add_test(NAME thinlto-duplicate-input
  COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
  "'${GOALLC_NOT_EXECUTABLE}' '$<TARGET_FILE:goallc-thinlto-test>' --save-temps=false --output-dir='${THIN_OUT}/duplicate' '${THIN_FIXTURES}/caller.ll' '${THIN_FIXTURES}/caller.ll' 2>&1 | '${GOALLC_FILECHECK_EXECUTABLE}' --check-prefix=DUPLICATE '${THIN_FIXTURES}/caller.ll'")

add_test(NAME thinlto-module-asm
  COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
  "'${GOALLC_FILECHECK_EXECUTABLE}' --check-prefix=ASM '${THIN_FIXTURES}/debug-caller.ll' --input-file='${THIN_DEBUG}/0.ll' && '${GOALLC_FILECHECK_EXECUTABLE}' --check-prefix=ASM '${THIN_FIXTURES}/debug-callee.ll' --input-file='${THIN_DEBUG}/1.ll'")
set_tests_properties(thinlto-module-asm PROPERTIES FIXTURES_REQUIRED thinlto-debug-ir)
foreach(ARCH IN ITEMS x86_64 aarch64)
  if(GOALLC_OBJVIEW_EXECUTABLE)
    add_test(NAME thinlto-cgo-${ARCH}-object
      COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
      "'${GOALLC_OBJVIEW_EXECUTABLE}' -json '${THIN_DEBUG}/${ARCH}.o' | '${GOALLC_FILECHECK_EXECUTABLE}' --check-prefix=CGO '${THIN_FIXTURES}/debug-caller.ll'")
    set_tests_properties(thinlto-cgo-${ARCH}-object PROPERTIES
      FIXTURES_REQUIRED thinlto-debug-${ARCH}-obj)
  endif()
endforeach()
add_test(NAME thinlto-symver-rejected
  COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
  "'${GOALLC_NOT_EXECUTABLE}' '$<TARGET_FILE:goallc-thinlto-test>' --emit-objects --save-temps=false --output-dir='${THIN_OUT}/symver' '${THIN_FIXTURES}/symver.ll' 2>&1 | '${GOALLC_FILECHECK_EXECUTABLE}' '${THIN_FIXTURES}/symver.ll'")

# The production plugin supplies final-layout inline anchors. Exercise symbol
# annotations with no source GlobalValue on both GoObj backends.
foreach(ARCH IN ITEMS x86_64 aarch64)
  add_test(NAME thinlto-debug-symbol-${ARCH}
    COMMAND "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      "-mtriple=${ARCH}-unknown-linux-goobj" -filetype=obj
      -o "${CMAKE_CURRENT_BINARY_DIR}/debug-symbol-${ARCH}.o"
      "${THIN_FIXTURES}/debug-symbol.ll")
  set_tests_properties(thinlto-debug-symbol-${ARCH} PROPERTIES
    FIXTURES_SETUP thinlto-debug-symbol-${ARCH}-obj)
  if(GOALLC_OBJVIEW_EXECUTABLE)
    add_test(NAME thinlto-debug-symbol-${ARCH}-object
      COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
      "'${GOALLC_OBJVIEW_EXECUTABLE}' -json '${CMAKE_CURRENT_BINARY_DIR}/debug-symbol-${ARCH}.o' | '${GOALLC_FILECHECK_EXECUTABLE}' --check-prefix=OBJ '${THIN_FIXTURES}/debug-symbol.ll'")
    set_tests_properties(thinlto-debug-symbol-${ARCH}-object PROPERTIES
      FIXTURES_REQUIRED thinlto-debug-symbol-${ARCH}-obj)
  endif()
endforeach()
add_test(NAME thinlto-debug-symbol-invalid
  COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
  "sed 's/i32 7/i64 4294967296/' '${THIN_FIXTURES}/debug-symbol.ll' > '${CMAKE_CURRENT_BINARY_DIR}/debug-symbol-bad.ll' && '${GOALLC_NOT_EXECUTABLE}' --crash '${GOALLC_LLC_EXECUTABLE}' '-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>' -mtriple=x86_64-unknown-linux-goobj -filetype=obj -o /dev/null '${CMAKE_CURRENT_BINARY_DIR}/debug-symbol-bad.ll' 2>&1 | '${GOALLC_FILECHECK_EXECUTABLE}' --check-prefix=BAD '${THIN_FIXTURES}/debug-symbol.ll'")

# Verify that parallel in-process codegen uses each module's own configuration.
set(THIN_PARALLEL "${CMAKE_CURRENT_BINARY_DIR}/thinlto-parallel")
add_test(NAME thinlto-parallel-objects
  COMMAND "$<TARGET_FILE:goallc-thinlto-test>" --emit-objects --jobs=4
    "--output-dir=${THIN_PARALLEL}"
    "${THIN_FIXTURES}/debug-caller.ll" "${THIN_FIXTURES}/debug-callee.ll")
set_tests_properties(thinlto-parallel-objects PROPERTIES FIXTURES_SETUP thinlto-parallel)
if(GOALLC_OBJVIEW_EXECUTABLE)
  foreach(INDEX RANGE 0 1)
    if(INDEX EQUAL 0)
      set(FIXTURE debug-caller.ll)
    else()
      set(FIXTURE debug-callee.ll)
    endif()
    add_test(NAME thinlto-parallel-owner-${INDEX}
      COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
      "'${GOALLC_OBJVIEW_EXECUTABLE}' -json '${THIN_PARALLEL}/${INDEX}.o' | '${GOALLC_FILECHECK_EXECUTABLE}' --check-prefix=OWNER '${THIN_FIXTURES}/${FIXTURE}'")
    set_tests_properties(thinlto-parallel-owner-${INDEX} PROPERTIES FIXTURES_REQUIRED thinlto-parallel)
  endforeach()
endif()

set(THIN_MARKERS "${CMAKE_CURRENT_BINARY_DIR}/thinlto-markers")
add_test(NAME thinlto-markers
  COMMAND "$<TARGET_FILE:goallc-thinlto-test>" --emit-objects
    "--output-dir=${THIN_MARKERS}" "${THIN_FIXTURES}/markers.ll")
set_tests_properties(thinlto-markers PROPERTIES FIXTURES_SETUP thinlto-markers)
if(GOALLC_OBJVIEW_EXECUTABLE)
  add_test(NAME thinlto-markers-object
    COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
    "'${GOALLC_OBJVIEW_EXECUTABLE}' -json '${THIN_MARKERS}/0.o' | '${GOALLC_FILECHECK_EXECUTABLE}' --check-prefix=OBJ '${THIN_FIXTURES}/markers.ll'")
  set_tests_properties(thinlto-markers-object PROPERTIES FIXTURES_REQUIRED thinlto-markers)
endif()

# Standard LTO normally deletes non-prevailing definitions. Go retains both
# package slots until its native loader resolves BSS and initialized storage.
set(THIN_DUP "${CMAKE_CURRENT_BINARY_DIR}/thinlto-duplicate-definitions")
foreach(ORDER IN ITEMS forward reverse)
  if(ORDER STREQUAL "forward")
    set(FIRST 0)
    set(SECOND 1)
  else()
    set(FIRST 1)
    set(SECOND 0)
  endif()
  add_test(NAME thinlto-duplicate-definitions-${ORDER}
    COMMAND "$<TARGET_FILE:goallc-thinlto-test>" --emit-objects --jobs=4
      "--output-dir=${THIN_DUP}-${ORDER}"
      "${THIN_FIXTURES}/duplicate-${FIRST}.ll"
      "${THIN_FIXTURES}/duplicate-${SECOND}.ll")
  set_tests_properties(thinlto-duplicate-definitions-${ORDER} PROPERTIES
    FIXTURES_SETUP thinlto-duplicate-definitions-${ORDER})
  if(GOALLC_OBJVIEW_EXECUTABLE)
    foreach(INDEX RANGE 0 1)
      if(INDEX EQUAL 0)
        set(FIXTURE ${FIRST})
      else()
        set(FIXTURE ${SECOND})
      endif()
      add_test(NAME thinlto-duplicate-definitions-${ORDER}-${INDEX}-object
        COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
        "'${GOALLC_OBJVIEW_EXECUTABLE}' -json '${THIN_DUP}-${ORDER}/${INDEX}.o' | '${GOALLC_FILECHECK_EXECUTABLE}' --check-prefix=OBJ '${THIN_FIXTURES}/duplicate-${FIXTURE}.ll' && '${GOALLC_FILECHECK_EXECUTABLE}' --check-prefix=IR --implicit-check-not=__goallc_lto. '${THIN_FIXTURES}/duplicate-${FIXTURE}.ll' --input-file='${THIN_DUP}-${ORDER}/${INDEX}.ll'")
      set_tests_properties(thinlto-duplicate-definitions-${ORDER}-${INDEX}-object PROPERTIES
        FIXTURES_REQUIRED thinlto-duplicate-definitions-${ORDER})
    endforeach()
  endif()
endforeach()

add_test(NAME thinlto-empty-package
  COMMAND "$<TARGET_FILE:goallc-thinlto-test>" --emit-objects --save-temps=false
    "--output-dir=${THIN_OUT}/empty" "${THIN_FIXTURES}/empty.ll")

add_executable(goallc-lto-transform-test testdata/thinlto/module-transform.cpp)
llvm_update_compile_flags(goallc-lto-transform-test)
target_compile_features(goallc-lto-transform-test PRIVATE cxx_std_17)
target_link_libraries(goallc-lto-transform-test PRIVATE LLVM)
foreach(MODE IN ITEMS full thin)
  add_test(NAME lto-module-transform-${MODE}
    COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
    "'$<TARGET_FILE:goallc-lto-transform-test>' ${MODE} | '${GOALLC_FILECHECK_EXECUTABLE}' --check-prefix=TRANSFORM '${THIN_FIXTURES}/module-transform.check'")
  add_test(NAME lto-module-transform-${MODE}-stop
    COMMAND "$<TARGET_FILE:goallc-lto-transform-test>" ${MODE} stop)
endforeach()
