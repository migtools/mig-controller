#!/bin/bash

R="\e[31m"
G="\e[32m"
N="\e[39m"

error() {
  echo -e "${R}$1${N}"
}

success() {
  echo -e "${G}$1${N}"
}

info() {
  echo -e "${N}$1${N}"
}

push_loc() {
  pushd ${CAM_E2E_REPO} &> /dev/null
}

pop_loc() {
  popd ${CAM_E2E_REPO} &> /dev/null
}

USAGE="$(basename "$0") [-h] [-l] [-t TEST_NAME] [-n NUMBER_OF_TESTS] [-e EXCLUDE_TESTS] [-d LOG_DIR] --- Runs MTC QE test(s)
 
Options :
  -h    Shows this help
  -l    Lists all available QE tests
  -n    Specifies number of random tests to run 
        Ignored when -t is specified
  -t    Specifies names of QE tests to run 
        Must be comma separated list of names
  -e    Excludes tests from random choice
        Must be comma separated list of names
        Ignored when -t is specified	
  -d    Specifies directory to store execution logs

Example Usage :
  1. Run 1 random QE test    : $(basename "$0")
  2. Run 2 random QE tests   : $(basename "$0") -n 2
  3. List all QE tests       : $(basename "$0") -l
  4. Run specific QE test(s) : $(basename "$0") -t ocp-24872-mssql,ocp-34824-decrease-data
"

# default arguments
CLEANUP=false
LIST=false
TESTS=""
NUMBER_OF_TESTS=1
EXCLUDE_TESTS="ocp-28021-istags,ocp-34867-imagestreamsstage,ocp-27335-velero-version,ocp-24769-cakephp"
TEMP_DIR="/tmp/"

check_kubeconfig() {
  if [[ -z ${KUBECONFIG} ]]; then
    error "Env var 'KUBECONFIG' must be set."
    exit 1
  else
    oc get project openshift-migration &> /dev/null
    if [[ $? -ne 0 ]]; then
      error "Namespace openshift-migration not found"
      exit 1
    fi
    clusters=$(oc get migclusters -n openshift-migration -o name)
    source_found=false
    destination_found=false
    for migcluster in ${clusters}; do
      cluster=$(oc get -n openshift-migration ${migcluster} -o go-template='{{ .spec.isHostCluster }}{{ println }}')
      if [[ ${cluster} == 'true' ]]; then
        destination_found=true
      else
        source_found=true
      fi
    done
    if [ $source_found != true ] || [ $destination_found != true ]; then
      error "Both the source and the destination MigCluster must be set up."
      exit 1
    fi
  fi
}

check_repo() {
  if [[ -z ${CAM_E2E_REPO} ]]; then
    error "Env var 'CAM_E2E_REPO' must be set. Must be absolute path to E2E repo."
    exit 1
  fi
  if [[ ! -d ${CAM_E2E_REPO} ]]; then
    error "Path 'CAM_E2E_REPO' must exist."
    exit 1
  fi
  if [[ ! -d ${CAM_E2E_REPO}/.git ]]; then
    error "Please clone https://gitlab.cee.redhat.com/app-mig/cam-e2e-qe to CAM_E2E_REPO location."
    exit 1
  fi
}

list_tests() {
  check_repo
  push_loc
  for f in $(ls | grep -E '^(ocp|max)(-[A-Z])*(-[0-9])*(-[A-Za-z]+)*'); do
    printf '%s\n' "${f%.yml}"
  done | column
  pop_loc
  exit
}

apply_filters() {
  filtered_tests=()
  tests=( "$@" )
  for test in "${tests[@]%.yml}"
  do
    if [[ ! ${EXCLUDE_TESTS} =~ .*"${test}".* ]]; then
      filtered_tests+=( "${test}" )
    fi
  done
}

# run a test randomly
run_random_tests() {
  check_repo
  check_kubeconfig
  push_loc
  all_tests=""
  randomized_tests=( $(for f in $(ls | grep -E '^(ocp|max)(-[A-Z])*(-[0-9])*(-[A-Za-z]+)*'); do
    echo "${f}"
  done | shuf | tr -d " ") )
  apply_filters ${randomized_tests[@]}
  info "Running ${NUMBER_OF_TESTS} random test(s)"
  for i in $(seq 0 $(($NUMBER_OF_TESTS - 1))); do
    TESTS="${TESTS}${filtered_tests[${i}]},"
  done
  run_tests
  pop_loc
}

run_tests() {
  push_loc
  tests_to_run=()
  IFS=', ' read -r -a tests_to_run <<< "$TESTS"
  for test in "${tests_to_run[@]}"; do
    run_test $test
    info "------------------------"
  done
  pop_loc
}

run_test() {
  tstamp=$(date +%F)
  test_to_run=$1
  test_output=${TEMP_DIR}/${tstamp}-${RANDOM}-${test_to_run}.log
  if [ -f "${test_to_run}.yml" ]; then
    info "Running test ${test_to_run}.yml"
    ansible-playbook -i inventory.cam.yml -e @config/defaults.yml ${test_to_run}.yml &> ${test_output}
    rc=$?
    if [[ $rc -ne 0 ]]; then
      error "FAILED ${test_to_run}"
      migration_name=$(cat ${test_output} | grep "Check if" | head -n1 | sed -r "s/.*migration (.*) is completed.*/\1/")
      error "Check MigMigration ${migration_name} for details..."
    else
      success "OK ${test_to_run}"
    fi
    info "Full execution logs stored in ${test_output}"
  else
    error "Test ${test_to_run} not found. Make sure test exists by running '$(basename "$0") -l'"
  fi
}

while getopts ':hlt:n:e:d:' option; do
  case "$option" in
    h) echo "$USAGE"
       exit
       ;;
    l) list_tests
       ;; 
    t) TESTS=$OPTARG
       ;;
    n) NUMBER_OF_TESTS=$OPTARG
       ;;
    e) EXCLUDE_TESTS=$OPTARG
       ;;
    d) TEMP_DIR=$OPTARG
       ;;
    :) error "missing argument for -%s\n" "$OPTARG" >&2
       echo "$USAGE" >&2
       exit 1
       ;;
   \?) error "illegal option: -%s\n" "$OPTARG" >&2
       echo "$USAGE" >&2
       exit 1
       ;;
  esac
done
shift $((OPTIND -1))

len=${#TESTS}

if [[ ${len} -eq 0 ]]; then
  run_random_tests
else
  run_tests
fi
