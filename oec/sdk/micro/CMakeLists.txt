cmake_minimum_required(VERSION 3.20)
project(oecm C)

# Host build of the micro SDK core (ADR-0030): the library, the Unity tests
# with ASan/UBSan and the payload emitter. The ESP-IDF component uses its
# own CMakeLists.txt in port/esp-idf.

set(CMAKE_C_STANDARD 17)
set(CMAKE_C_STANDARD_REQUIRED ON)
set(CMAKE_C_EXTENSIONS OFF)

option(OECM_SANITIZE "Build with AddressSanitizer and UndefinedBehaviorSanitizer" ON)

set(OECM_WARNINGS
    -Wall -Wextra -Wpedantic -Wconversion -Wsign-conversion -Wshadow -Wcast-qual
    -Wstrict-prototypes -Wmissing-prototypes -Wvla -Wformat=2 -Werror)

if(OECM_SANITIZE)
  set(OECM_SAN -fsanitize=address,undefined -fno-sanitize-recover=all -fno-omit-frame-pointer)
endif()

file(GLOB OECM_SOURCES CONFIGURE_DEPENDS ${CMAKE_CURRENT_SOURCE_DIR}/src/*.c)
add_library(oecm STATIC ${OECM_SOURCES})
target_include_directories(oecm PUBLIC include PRIVATE src)
target_compile_options(oecm PRIVATE ${OECM_WARNINGS} ${OECM_SAN})

# The core without sanitizers: the symbol allowlist in check.sh.
add_library(oecm_plain STATIC ${OECM_SOURCES})
target_include_directories(oecm_plain PUBLIC include PRIVATE src)
target_compile_options(oecm_plain PRIVATE ${OECM_WARNINGS} -Os -ffreestanding -fno-builtin -fno-stack-protector)

add_library(unity STATIC third_party/unity/unity.c)
target_include_directories(unity PUBLIC third_party/unity)
target_compile_definitions(unity PUBLIC UNITY_INCLUDE_DOUBLE UNITY_SUPPORT_64)
target_compile_options(unity PRIVATE ${OECM_SAN})

add_library(fake_pal STATIC tests/fake_pal.c)
target_link_libraries(fake_pal PUBLIC oecm)
target_compile_options(fake_pal PRIVATE ${OECM_WARNINGS} ${OECM_SAN})

enable_testing()
foreach(t il json device)
  add_executable(test_${t} tests/test_${t}.c)
  target_include_directories(test_${t} PRIVATE src tests)
  target_link_libraries(test_${t} PRIVATE fake_pal unity)
  # Unity and the generated vectors are tested code, not the core.
  target_compile_options(test_${t} PRIVATE ${OECM_WARNINGS} -Wno-missing-prototypes
                         -Wno-unused-const-variable ${OECM_SAN})
  target_link_options(test_${t} PRIVATE ${OECM_SAN})
  add_test(NAME ${t} COMMAND test_${t})
endforeach()

add_executable(oecm-emit tests/emit.c)
target_link_libraries(oecm-emit PRIVATE fake_pal)
target_compile_options(oecm-emit PRIVATE ${OECM_WARNINGS} ${OECM_SAN})
target_link_options(oecm-emit PRIVATE ${OECM_SAN})

add_executable(hello_device examples/host/hello_device.c)
target_link_libraries(hello_device PRIVATE oecm)
target_compile_options(hello_device PRIVATE ${OECM_WARNINGS} ${OECM_SAN})
target_link_options(hello_device PRIVATE ${OECM_SAN})
