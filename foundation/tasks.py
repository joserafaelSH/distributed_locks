import subprocess
import sys
from enum import StrEnum, auto

class Command(StrEnum):
    CLUSTER_UP = auto()
    CLUSTER_DOWN = auto()
    HELP = auto()


def clusterDown():
    print("Deleting k3d cluster...")
    result = subprocess.run(["k3d", "cluster", "delete"])
    if result.returncode != 0:
        print("Failed to delete cluster.", file=sys.stderr)
        sys.exit(result.returncode)
    print("Cluster deleted successfully!")

def clusterUp():
    print("Creating k3d cluster...")
    result = subprocess.run(["k3d", "cluster", "create", "--config", "cluster/k3d-config.yaml"])
    if result.returncode != 0:
        print("Failed to create cluster.", file=sys.stderr)
        sys.exit(result.returncode)
    subprocess.run(["kubectl", "cluster-info"])
    print("Cluster created successfully!")

def main(command: str):
    match command:
        case Command.CLUSTER_UP:
            clusterUp()
        case Command.CLUSTER_DOWN:
            clusterDown()
        case Command.HELP:
            print("Usage: python tasks.py <command>")
            print("Commands:")
            print("  cluster_up")
            print("  cluster_down")
            print("  help")
        case _ :
            print(f"Unknown command: {command}")
            sys.exit(1)

if __name__ == "__main__":
    # get system arguments
    if len(sys.argv) < 2:
        print("Usage: python tasks.py <command>")
        sys.exit(1)
    command = sys.argv[1]
    main(command)