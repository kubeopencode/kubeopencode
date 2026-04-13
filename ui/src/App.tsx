import { BrowserRouter, Routes, Route } from 'react-router-dom';
import Layout from './components/Layout';
import { NamespaceProvider } from './contexts/NamespaceContext';
import DashboardPage from './pages/DashboardPage';
import TasksPage from './pages/TasksPage';
import TaskDetailPage from './pages/TaskDetailPage';
import TaskCreatePage from './pages/TaskCreatePage';
import AgentsPage from './pages/AgentsPage';
import AgentDetailPage from './pages/AgentDetailPage';
import AgentCreatePage from './pages/AgentCreatePage';
import AgentTemplatesPage from './pages/AgentTemplatesPage';
import AgentTemplateDetailPage from './pages/AgentTemplateDetailPage';
import AgentTemplateCreatePage from './pages/AgentTemplateCreatePage';
import CronTasksPage from './pages/CronTasksPage';
import CronTaskCreatePage from './pages/CronTaskCreatePage';
import CronTaskDetailPage from './pages/CronTaskDetailPage';
import ConfigPage from './pages/ConfigPage';
import NotFoundPage from './pages/NotFoundPage';

function App() {
  return (
    <BrowserRouter>
      <NamespaceProvider>
      <Routes>
        <Route path="/" element={<Layout />}>
          <Route index element={<DashboardPage />} />
          <Route path="tasks" element={<TasksPage />} />
          <Route path="tasks/create" element={<TaskCreatePage />} />
          <Route path="tasks/:namespace/:name" element={<TaskDetailPage />} />
          <Route path="crontasks" element={<CronTasksPage />} />
          <Route path="crontasks/create" element={<CronTaskCreatePage />} />
          <Route path="crontasks/:namespace/:name" element={<CronTaskDetailPage />} />
          <Route path="agents" element={<AgentsPage />} />
          <Route path="agents/create" element={<AgentCreatePage />} />
          <Route path="agents/:namespace/:name" element={<AgentDetailPage />} />
          <Route path="templates" element={<AgentTemplatesPage />} />
          <Route path="templates/create" element={<AgentTemplateCreatePage />} />
          <Route path="templates/:namespace/:name" element={<AgentTemplateDetailPage />} />
          <Route path="config" element={<ConfigPage />} />
          <Route path="*" element={<NotFoundPage />} />
        </Route>
      </Routes>
      </NamespaceProvider>
    </BrowserRouter>
  );
}

export default App;
