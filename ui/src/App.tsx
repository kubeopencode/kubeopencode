import { BrowserRouter, Routes, Route } from 'react-router-dom';
import Layout from './components/Layout';
import DashboardPage from './pages/DashboardPage';
import TasksPage from './pages/TasksPage';
import TaskDetailPage from './pages/TaskDetailPage';
import TaskCreatePage from './pages/TaskCreatePage';
import AgentsPage from './pages/AgentsPage';
import AgentDetailPage from './pages/AgentDetailPage';
import AgentTemplatesPage from './pages/AgentTemplatesPage';
import AgentTemplateDetailPage from './pages/AgentTemplateDetailPage';

function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Layout />}>
          <Route index element={<DashboardPage />} />
          <Route path="tasks" element={<TasksPage />} />
          <Route path="tasks/create" element={<TaskCreatePage />} />
          <Route path="tasks/:namespace/:name" element={<TaskDetailPage />} />
          <Route path="agents" element={<AgentsPage />} />
          <Route path="agents/:namespace/:name" element={<AgentDetailPage />} />
          <Route path="templates" element={<AgentTemplatesPage />} />
          <Route path="templates/:namespace/:name" element={<AgentTemplateDetailPage />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}

export default App;
