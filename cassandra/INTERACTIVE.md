# Interactive Node Manager

The interactive mode provides a tmux-like interface for managing Cassandra nodes with keyboard shortcuts.

## Usage

```bash
# Start interactive mode
go run . interactive
# or
./cassandra interactive
```

## Keyboard Shortcuts

### Normal Mode

- **C** - Create a new node
  - Automatically assigns the next available port (starting from 50051)
  - Node ID is auto-generated (node-1, node-2, etc.)

- **D** - Enter delete mode
  - Shows a numbered list of all running nodes
  - Use arrow keys or number keys to select

- **Q** or **Ctrl+C** - Quit
  - Stops all running nodes gracefully before exiting

### Delete Mode

When you press **D**, you enter delete mode where you can:

- **↑/↓** or **K/J** - Navigate through the list
- **1-9** - Jump directly to a node by number and delete it immediately
- **Enter** or **Space** - Delete the currently selected node
- **Esc** - Cancel and return to normal mode

## Features

- **Auto-refresh**: The node list updates automatically every second
- **Auto-port assignment**: New nodes get the next available port automatically
- **Visual feedback**: Selected nodes in delete mode are highlighted in red
- **Error handling**: Errors are displayed at the top of the screen

## Example Workflow

1. Start interactive mode: `go run . interactive`
2. Press **C** to create node-1 (port 50051)
3. Press **C** again to create node-2 (port 50052)
4. Press **C** again to create node-3 (port 50053)
5. Press **D** to enter delete mode
6. Press **2** to immediately delete node-2
7. Or use arrow keys to navigate and press Enter to delete
8. Press **Q** to quit (stops all remaining nodes)

## Node Information

Each node displays:
- **Node ID**: Auto-generated identifier (node-1, node-2, etc.)
- **Port**: The port the node is listening on

Nodes run in server mode by default and are ready to receive heartbeats from other nodes.

