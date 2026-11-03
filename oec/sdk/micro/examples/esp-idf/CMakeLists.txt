# Example firmware of the OEC Micro SDK (ADR-0030 §8, C-096): the simulated
# temperature sensor of examples/sensor as a smart device on ESP32/ESP32-S3,
# or in Espressif QEMU (sdkconfig.qemu). Build with build.sh.
cmake_minimum_required(VERSION 3.22)
include($ENV{IDF_PATH}/tools/cmake/project.cmake)
idf_build_set_property(MINIMAL_BUILD ON)
set(PROJECT_VER "0.1.0")
project(oecm_example)
