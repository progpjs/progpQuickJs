cmake_minimum_required(VERSION 3.25)

project(quickjs C)

option(LEAK_TRIGGER "Add a leak trigger" FALSE)

file(STRINGS "src/VERSION" CONFIG_VERSION)

include_directories(src)

if (CMAKE_CXX_COMPILER_ID MATCHES "Clang")
    set(CONFIG_CC clang)
else()
    set(CONFIG_CC gcc)
endif()

set(COMMON_FLAGS -D_GNU_SOURCE -DCONFIG_VERSION=\"${CONFIG_VERSION}\" -DCONFIG_CC=\"${CONFIG_CC}\" -DCONFIG_PREFIX=\"/usr/local\" -DCONFIG_BIGNUM)

FILE(GLOB QUICKJS_LIB_SOURCES  "src/*.c")
set(QJS_LIB_SOURCES ${QUICKJS_LIB_SOURCES})

set(CMAKE_LIBRARY_OUTPUT_DIRECTORY ${CMAKE_SOURCE_DIR}/${CMAKE_SYSTEM_NAME})
set(CMAKE_ARCHIVE_OUTPUT_DIRECTORY ${CMAKE_SOURCE_DIR}/${CMAKE_SYSTEM_NAME})

if (CMAKE_BUILD_TYPE STREQUAL "Debug")
    set(COMMON_FLAGS ${COMMON_FLAGS} -DDUMP_LEAKS)
endif (CMAKE_BUILD_TYPE STREQUAL "Debug")

if (LEAK_TRIGGER)
    # Use printf as leak_trigger
    set(COMMON_FLAGS ${COMMON_FLAGS} -Dprintf=leak_trigger)
endif (LEAK_TRIGGER)

add_library(quickjs STATIC ${QUICKJS_LIB_SOURCES})
target_compile_options(quickjs PRIVATE ${COMMON_FLAGS})
target_include_directories(quickjs PUBLIC quickjs)