#!/bin/bash

# Output command before executing
set -x

# Exit on error
set -e

# Source environment variables of the jenkins slave
# that might interest this worker.
function load_jenkins_vars() {
  if [ -e "jenkins-env" ]; then
    cat jenkins-env \
      | grep -E "(JENKINS_URL|GIT_BRANCH|GIT_COMMIT|BUILD_NUMBER|ghprbSourceBranch|ghprbActualCommit|BUILD_URL|ghprbPullId)=" \
      | sed 's/^/export /g' \
      > ~/.jenkins-env
    source ~/.jenkins-env
  fi
}

function install_deps() {
  # We need to disable selinux for now, XXX
  /usr/sbin/setenforce 0

  # Get all the deps in
  yum -y install \
    docker \
    make \
    git \
    curl

  service docker start

  echo 'CICO: Dependencies installed'
}

function cleanup_env {
  EXIT_CODE=$?
  echo "CICO: Cleanup environment: Tear down test environment"
  make integration-test-env-tear-down
  echo "CICO: Exiting with $EXIT_CODE"
}

function prepare() {
  # Let's test
  make docker-start
  make docker-check-go-format
  make docker-deps
  make docker-analyze-go-code
  make docker-generate
  make docker-build
  echo 'CICO: Preparation complete'
}

function run_tests_without_coverage() {
  make docker-test-unit-no-coverage
  make integration-test-env-prepare
  trap cleanup_env EXIT

  # Check that postgresql container is healthy
  check_postgres_healthiness

  make docker-test-migration
  make docker-test-integration-no-coverage
  make docker-test-remote-no-coverage
  echo "CICO: ran tests without coverage"
}

function check_postgres_healthiness(){
  echo "CICO: Waiting for postgresql container to be healthy...";
  while ! docker ps | grep postgres_integration_test | grep -q healthy; do
    printf .;
    sleep 1 ;
  done;
  echo "CICO: postgresql container is HEALTHY!";
}

function run_tests_with_coverage() {
  CODECOV_TOKEN=ad12dad7-ebdc-47bc-a016-8c05fa7356bc

  # Run the unit tests that generate coverage information and upload the
  # results.
  make docker-test-unit
  cp tmp/$(ls --hide=*tmp tmp/coverage.unit.mode-*) coverage-unit_tests.txt
  bash <(curl -s https://codecov.io/bash) -X search -f coverage-unit_tests.txt -t $CODECOV_TOKEN -F unit_tests

  make integration-test-env-prepare
  trap cleanup_env EXIT

  # Check that postgresql container is healthy
  check_postgres_healthiness

  # Run the integration tests that generate coverage information and upload the
  # results.
  make docker-test-migration
  make docker-test-integration
  cp tmp/$(ls --hide=*tmp tmp/coverage.integration.mode-*) coverage-integration_tests.txt 
  bash <(curl -s https://codecov.io/bash) -X search -f coverage-integration_tests.txt -t $CODECOV_TOKEN -F integration_tests

  # Run the remote tests that generate coverage information and upload the
  # results.
  make docker-test-remote
  cp tmp/$(ls --hide=*tmp tmp/coverage.remote.mode-*) coverage-remote_tests.txt
  bash <(curl -s https://codecov.io/bash) -X search -f coverage-remote_tests.txt -t $CODECOV_TOKEN -F remote_tests

  # Output coverage
  make docker-coverage-all

  # Upload overall coverage to codecov.io
  cp tmp/coverage.mode* coverage-all.txt
  bash <(curl -s https://codecov.io/bash) -X search -f coverage-all.txt -t $CODECOV_TOKEN #-X fix

  echo "CICO: ran tests and uploaded coverage"
}

function tag_push() {
  TARGET=$1
  docker tag almighty-core-deploy $TARGET
  docker push $TARGET
}

function deploy() {
  # Let's deploy
  make docker-image-deploy

  TAG=$(echo $GIT_COMMIT | cut -c1-6)

  tag_push registry.devshift.net/almighty/almighty-core:$TAG
  tag_push registry.devshift.net/almighty/almighty-core:latest
  echo 'CICO: Image pushed, ready to update deployed app'
}

function cico_setup() {
  load_jenkins_vars;
  install_deps;
  prepare;
}
