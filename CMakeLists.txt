# This file allows compile the project as a standalone executable.

cmake_minimum_required(VERSION 3.10)
project(quickJsEngine)
set(CMAKE_CXX_STANDARD 17)

macro(project_printInfos)
    message("SYSTEM_ARCH: ${CMAKE_SYSTEM_NAME}")
    message("PROJECT_SOURCES_H: ${PROJECT_SOURCES_H}")
    message("PROJECT_SOURCES_C: ${PROJECT_SOURCES_C}")
    message("PROJECT_LIB_DIR: ${PROJECT_LIB_DIR}")
    message("PROJECT_INCLUDE_SEARCH: ${PROJECT_INCLUDE_SEARCH}")
endmacro()

FILE(GLOB PROJECT_SOURCES_H                 "${CMAKE_SOURCE_DIR}/*.h")
FILE(GLOB PROJECT_SOURCES_C                 "${CMAKE_SOURCE_DIR}/*.c")

set(PROJECT_LIB_DIR                         "${CMAKE_SOURCE_DIR}/libQuickJS/${CMAKE_SYSTEM_NAME}")
set(PROJECT_INCLUDE_DIR                     "${CMAKE_SOURCE_DIR}/libQuickJS/src")
set(PROJECT_COMPILE_OPTIONS                 -Wall -fPIC)
set(PROJECT_INCLUDE_SEARCH                  ${PROJECT_INCLUDE_DIR})

project_printInfos()

add_executable(quickJsEngine)
#
target_sources(quickJsEngine PRIVATE ${PROJECT_SOURCES_C})
target_link_libraries(quickJsEngine PRIVATE -lm -ldl -lpthread -lquickjs -L${PROJECT_LIB_DIR})
target_compile_options(quickJsEngine PRIVATE ${PROJECT_COMPILE_OPTIONS} -DPROGP_STANDALONE)
target_include_directories(quickJsEngine PRIVATE ${PROJECT_INCLUDE_SEARCH})