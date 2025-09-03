#!/bin/bash

# Comprehensive test scenarios for the backup operator
set -e

NAMESPACE_DEFAULT="default"
NAMESPACE_TEST="backup-test"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log() {
    echo -e "${BLUE}[$(date +'%H:%M:%S')]${NC} $1"
}

success() {
    echo -e "${GREEN}✅ $1${NC}"
}

warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

error() {
    echo -e "${RED}❌ $1${NC}"
}

# Function to wait for resource to be ready
wait_for_resource() {
    local resource_type=$1
    local resource_name=$2
    local namespace=${3:-default}
    local timeout=${4:-300}

    log "Waiting for $resource_type/$resource_name in namespace $namespace..."

    local interval=10
    local elapsed=0

    while [ $elapsed -lt $timeout ]; do
        if kubectl get $resource_type $resource_name -n $namespace >/dev/null 2>&1; then
            success "$resource_type/$resource_name is ready"
            return 0
        fi
        sleep $interval
        elapsed=$((elapsed + interval))
        echo "   Still waiting... ($elapsed/${timeout}s)"
    done

    error "Timeout waiting for $resource_type/$resource_name"
    return 1
}

# Function to check pod readiness
wait_for_pods() {
    local label_selector=$1
    local namespace=${2:-default}
    local timeout=${3:-300}

    log "Waiting for pods with label: $label_selector in namespace $namespace"
    kubectl wait --for=condition=Ready pod -l "$label_selector" -n $namespace --timeout=${timeout}s
}

# Main test function
run_test_scenario() {
    local scenario=$1

    case $scenario in
        "setup")
            log "🚀 Setting up general resources..."
            kubectl apply -f test/00_setup_general.yaml
            success "General setup complete"
            ;;

        "backup-config")
            log "⚙️  Creating BackupConfigs..."
            kubectl apply -f test/01_create_backup_config.yaml
            wait_for_resource backupconfig test-local-backup $NAMESPACE_DEFAULT
            wait_for_resource backupconfig test-nfs-backup $NAMESPACE_TEST
            success "BackupConfigs created"
            ;;

        "deploy-app")
            log "🚀 Deploying web application..."
            kubectl apply -f test/02_deploy_app.yaml
            wait_for_resource deployment web-app $NAMESPACE_DEFAULT
            wait_for_pods "app=web-app" $NAMESPACE_DEFAULT
            success "Web application deployed"
            ;;

        "deploy-db")
            log "🗄️  Deploying PostgreSQL database..."
            kubectl apply -f test/03_deploy_database.yaml
            wait_for_resource deployment postgres $NAMESPACE_TEST
            wait_for_pods "app=postgres" $NAMESPACE_TEST 600  # Give DB more time
            success "PostgreSQL database deployed"
            ;;

        "monitoring")
            log "📊 Deploying monitoring tools..."
            kubectl apply -f test/05_monitoring_tools.yaml
            wait_for_resource deployment backup-filebrowser $NAMESPACE_DEFAULT
            wait_for_pods "app=backup-filebrowser" $NAMESPACE_DEFAULT
            success "Monitoring tools deployed"
            ;;

        "restore-jobs")
            warning "Creating RestoreJobs (they may fail if no backups exist yet)..."
            kubectl apply -f test/04_manual_restore_jobs.yaml
            success "RestoreJobs created (check their status manually)"
            ;;

        "status")
            log "📋 Checking overall status..."
            echo ""
            echo "=== BackupConfigs ==="
            kubectl get backupconfig -A
            echo ""
            echo "=== BackupJobs ==="
            kubectl get backupjobs -A
            echo ""
            echo "=== RestoreJobs ==="
            kubectl get restorejobs -A
            echo ""
            echo "=== PVCs ==="
            kubectl get pvc -A | grep -E "(backup|postgres|web-app)"
            echo ""
            echo "=== Pods ==="
            kubectl get pods -A | grep -E "(web-app|postgres|backup|debug)"
            ;;

        "cleanup")
            log "🧹 Cleaning up test resources..."
            kubectl delete -f test/04_manual_restore_jobs.yaml --ignore-not-found=true
            kubectl delete -f test/05_monitoring_tools.yaml --ignore-not-found=true
            kubectl delete -f test/03_deploy_database.yaml --ignore-not-found=true
            kubectl delete -f test/02_deploy_app.yaml --ignore-not-found=true
            kubectl delete -f test/01_create_backup_config.yaml --ignore-not-found=true
            kubectl delete -f test/00_setup_general.yaml --ignore-not-found=true
            success "Cleanup complete"
            ;;

        "all")
            log "🎯 Running complete test scenario..."
            run_test_scenario "setup"
            sleep 5
            run_test_scenario "backup-config"
            sleep 5
            run_test_scenario "deploy-app"
            sleep 5
            run_test_scenario "deploy-db"
            sleep 10
            run_test_scenario "monitoring"
            sleep 5
            run_test_scenario "status"

            log "🎉 Complete test deployment finished!"
            echo ""
            echo "📋 Next steps:"
            echo "1. Deploy the operator: make deploy"
            echo "2. Watch operator logs: kubectl logs -n datarestor-operator-system deployment/datarestor-operator-controller-manager -f"
            echo "3. Monitor BackupJobs: kubectl get backupjobs -A -w"
            echo "4. Access filebrowser: kubectl port-forward svc/backup-filebrowser 8080:8080"
            echo "5. Check app: kubectl port-forward svc/web-app 8081:80"
            echo "6. Create restore jobs: ./test/test-scenarios.sh restore-jobs"
            ;;

        *)
            error "Unknown scenario: $scenario"
            echo ""
            echo "Available scenarios:"
            echo "  setup        - Deploy secrets and general resources"
            echo "  backup-config - Create BackupConfig resources"
            echo "  deploy-app   - Deploy web application"
            echo "  deploy-db    - Deploy PostgreSQL database"
            echo "  monitoring   - Deploy monitoring tools"
            echo "  restore-jobs - Create RestoreJob examples"
            echo "  status       - Show status of all resources"
            echo "  cleanup      - Remove all test resources"
            echo "  all          - Run complete deployment (excluding restore-jobs)"
            exit 1
            ;;
    esac
}

# Main execution
if [ $# -eq 0 ]; then
    echo "Usage: $0 <scenario>"
    run_test_scenario "help"
else
    run_test_scenario $1
fi
