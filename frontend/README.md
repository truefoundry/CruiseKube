# Nodes Charting React App

## How to Run

1. **Install dependencies:**

   ```bash
   npm install
   ```

2. **Serve `data.json` for local development:**

   The React app fetches `data.json` from the parent directory. You can serve it using a simple static server. In the project root (where `data.json` is), run:

   ```bash
   npx serve .
   ```

   This will serve files on `http://localhost:3000` (or another port if 3000 is taken). The React app will be available at `/nodes-charting` and will be able to fetch `data.json`.

3. **Start the React app:**

   In another terminal, run:

   ```bash
   cd nodes-charting
   npm start
   ```

   The app will open in your browser. It will refresh the chart every 3 seconds with the latest data from `data.json`.

## Notes
- Each bar represents a node. Each stack in the bar is a pod, with height equal to `requested_cpu`.
- Green = `continuousOptimization: false`, Yellow = `continuousOptimization: true`. 