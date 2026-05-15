import React, { useMemo, useState } from 'react';
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom';
import { useQuery, useMutation } from '@tanstack/react-query';
import api, {
  CreateAgentRequest,
  CreateAgentTemplateRequest,
  RegistryImageInfo,
  RegistryPluginInfo,
  RegistrySkillInfo,
  AssemblySkillInput,
  AssemblyPluginInput,
} from '../api/client';
import { useToast } from '../contexts/ToastContext';
import Breadcrumbs from '../components/Breadcrumbs';

type OutputKind = 'agent' | 'template';

const DEFAULT_WORKSPACE_DIR = '/workspace';
const DEFAULT_SERVICE_ACCOUNT = 'kubeopencode-agent';

function PhasePill({ phase }: { phase: string }) {
  const isReady = phase === 'Ready';
  return (
    <span
      className={`inline-flex items-center gap-1 text-[10px] px-1.5 py-0.5 rounded font-medium ${
        isReady
          ? 'bg-emerald-50 text-emerald-700 border border-emerald-200'
          : 'bg-amber-50 text-amber-700 border border-amber-200'
      }`}
    >
      <span className={`w-1.5 h-1.5 rounded-full ${isReady ? 'bg-emerald-500' : 'bg-amber-500'}`} />
      {isReady ? 'Ready' : 'Unavailable'}
    </span>
  );
}

function imageBelongsTo(img: RegistryImageInfo, slot: 'agent' | 'executor'): boolean {
  const category = (img.metadata?.category || '').toLowerCase();
  if (!category) return true;
  return category === slot;
}

function RegistryAssemblePage() {
  const { namespace, name } = useParams<{ namespace: string; name: string }>();
  const navigate = useNavigate();
  const { addToast } = useToast();
  const [searchParams] = useSearchParams();

  const { data: registry, isLoading, error } = useQuery({
    queryKey: ['registry', namespace, name],
    queryFn: () => api.getRegistry(namespace!, name!),
    enabled: !!namespace && !!name,
  });

  const initialOutput: OutputKind = searchParams.get('output') === 'agent' ? 'agent' : 'template';
  const [outputKind, setOutputKind] = useState<OutputKind>(initialOutput);
  const [resourceName, setResourceName] = useState('');
  const [workspaceDir, setWorkspaceDir] = useState(DEFAULT_WORKSPACE_DIR);
  const [serviceAccountName, setServiceAccountName] = useState(DEFAULT_SERVICE_ACCOUNT);
  const [agentImage, setAgentImage] = useState('');
  const [executorImage, setExecutorImage] = useState('');
  const [selectedSkills, setSelectedSkills] = useState<Set<string>>(new Set());
  const [selectedPlugins, setSelectedPlugins] = useState<Set<string>>(new Set());

  const images = registry?.images ?? [];
  const skills = registry?.skills ?? [];
  const plugins = registry?.plugins ?? [];

  const agentImageCandidates = useMemo(
    () => images.filter((img) => imageBelongsTo(img, 'agent')),
    [images]
  );
  const executorImageCandidates = useMemo(
    () => images.filter((img) => imageBelongsTo(img, 'executor')),
    [images]
  );

  const toggle = (setter: React.Dispatch<React.SetStateAction<Set<string>>>, key: string) => {
    setter((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  };

  const buildSkillsInput = (): AssemblySkillInput[] => {
    return Array.from(selectedSkills)
      .map((skillName) => skills.find((s) => s.name === skillName))
      .filter((s): s is RegistrySkillInfo => !!s && !!s.repository)
      .map((s) => ({
        name: s.name,
        git: {
          repository: s.repository!,
          ref: s.ref || undefined,
          path: s.path || undefined,
          names: s.names && s.names.length > 0 ? s.names : undefined,
        },
      }));
  };

  const buildPluginsInput = (): AssemblyPluginInput[] => {
    return Array.from(selectedPlugins)
      .map((pluginName) => plugins.find((p) => p.name === pluginName))
      .filter((p): p is RegistryPluginInfo => !!p)
      .map((p) => ({
        name: p.package,
        target: p.target || undefined,
      }));
  };

  const createMutation = useMutation({
    mutationFn: async () => {
      const skillsInput = buildSkillsInput();
      const pluginsInput = buildPluginsInput();

      if (outputKind === 'template') {
        const req: CreateAgentTemplateRequest = {
          name: resourceName,
          workspaceDir: workspaceDir || undefined,
          serviceAccountName: serviceAccountName || undefined,
          agentImage: agentImage || undefined,
          executorImage: executorImage || undefined,
          skills: skillsInput.length > 0 ? skillsInput : undefined,
          plugins: pluginsInput.length > 0 ? pluginsInput : undefined,
        };
        return await api.createAgentTemplate(namespace!, req).then((r) => ({
          kind: 'template' as const,
          namespace: r.namespace,
          name: r.name,
        }));
      }

      const req: CreateAgentRequest = {
        name: resourceName,
        workspaceDir: workspaceDir || undefined,
        serviceAccountName: serviceAccountName || undefined,
        agentImage: agentImage || undefined,
        executorImage: executorImage || undefined,
        skills: skillsInput.length > 0 ? skillsInput : undefined,
        plugins: pluginsInput.length > 0 ? pluginsInput : undefined,
      };
      return await api.createAgent(namespace!, req).then((r) => ({
        kind: 'agent' as const,
        namespace: r.namespace,
        name: r.name,
      }));
    },
    onSuccess: (result) => {
      const label = result.kind === 'template' ? 'AgentTemplate' : 'Agent';
      addToast(`${label} "${result.name}" created from registry`, 'success');
      navigate(
        result.kind === 'template'
          ? `/templates/${result.namespace}/${result.name}`
          : `/agents/${result.namespace}/${result.name}`
      );
    },
    onError: (err: Error) => {
      addToast(`Failed to create: ${err.message}`, 'error');
    },
  });

  const isValid =
    !!resourceName && !!workspaceDir && !!serviceAccountName && !!executorImage;

  if (isLoading) {
    return (
      <div className="animate-fade-in">
        <div className="bg-white rounded-xl border-0 shadow-card p-8 text-stone-400 text-sm">Loading registry…</div>
      </div>
    );
  }

  if (error || !registry) {
    return (
      <div className="bg-red-50 border border-red-200 rounded-xl p-6 animate-fade-in">
        <h3 className="font-display text-base font-semibold text-red-800 mb-2">Registry Not Found</h3>
        <p className="text-sm text-red-600 mb-4">{(error as Error)?.message || 'Not found'}</p>
        <Link
          to="/registries"
          className="inline-flex items-center px-4 py-2 text-sm font-medium text-red-700 bg-red-100 rounded-lg hover:bg-red-200"
        >
          Back to Registries
        </Link>
      </div>
    );
  }

  const labelClass = 'block text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider mb-1.5';
  const inputClass = 'block w-full px-3 py-2 rounded-lg border border-stone-200 shadow-sm focus:border-primary-500 focus:ring-primary-500 text-sm text-stone-700 placeholder:text-stone-300';
  const monoInputClass = `${inputClass} font-mono placeholder:font-body`;

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    createMutation.mutate();
  };

  const renderImageCard = (img: RegistryImageInfo, picked: boolean, onPick: () => void) => {
    const isUnavailable = img.phase !== 'Ready';
    return (
      <button
        key={img.name}
        type="button"
        onClick={onPick}
        className={`text-left rounded-lg p-3 border transition-all ${
          picked
            ? 'border-primary-500 ring-2 ring-primary-100 bg-primary-50/40'
            : 'border-stone-200 bg-white hover:border-stone-300'
        }`}
      >
        <div className="flex items-start justify-between gap-2">
          <span className="text-sm font-semibold text-stone-800">{img.name}</span>
          <PhasePill phase={img.phase} />
        </div>
        <p className="mt-1 text-[11px] text-stone-500 font-mono break-all">{img.image}</p>
        {img.metadata?.description && (
          <p className="mt-1.5 text-[11px] text-stone-400 line-clamp-2">{img.metadata.description}</p>
        )}
        {isUnavailable && (
          <p className="mt-1.5 text-[11px] text-amber-700">
            Asset is Unavailable — selection allowed but may fail at runtime.
          </p>
        )}
      </button>
    );
  };

  const renderAssetRow = (
    key: string,
    selected: boolean,
    onToggle: () => void,
    title: string,
    subtitle: string | undefined,
    phase: string,
  ) => {
    const isUnavailable = phase !== 'Ready';
    return (
      <label
        key={key}
        className={`flex items-start gap-3 p-3 rounded-lg border cursor-pointer transition-all ${
          selected
            ? 'border-primary-500 ring-1 ring-primary-100 bg-primary-50/30'
            : 'border-stone-200 bg-white hover:border-stone-300'
        }`}
      >
        <input
          type="checkbox"
          checked={selected}
          onChange={onToggle}
          className="mt-1 rounded border-stone-300 text-primary-600 focus:ring-primary-500"
        />
        <div className="flex-1 min-w-0">
          <div className="flex items-center justify-between gap-2">
            <span className="text-sm font-medium text-stone-800 truncate">{title}</span>
            <PhasePill phase={phase} />
          </div>
          {subtitle && <p className="mt-0.5 text-[11px] text-stone-500 font-mono break-all">{subtitle}</p>}
          {isUnavailable && selected && (
            <p className="mt-1 text-[11px] text-amber-700">
              Selected an Unavailable asset — runtime may fail.
            </p>
          )}
        </div>
      </label>
    );
  };

  return (
    <div className="animate-fade-in">
      <Breadcrumbs
        items={[
          { label: 'Registries', to: '/registries' },
          { label: namespace!, isNamespace: true },
          { label: name!, to: `/registries/${namespace}/${name}` },
          { label: 'Assemble' },
        ]}
      />

      <div className="bg-white rounded-xl border-0 shadow-card max-w-4xl">
        <div className="px-6 py-5 border-b border-stone-100">
          <h2 className="font-display text-xl font-bold text-stone-900">Assemble from Registry</h2>
          <p className="text-sm text-stone-400 mt-0.5">
            Pick assets from <span className="font-mono">{registry.name}</span> to create an Agent or AgentTemplate.
          </p>
        </div>

        <form onSubmit={handleSubmit} className="px-6 py-5 space-y-6">
          {/* Output kind toggle */}
          <div>
            <span className={labelClass}>Output Resource</span>
            <div className="inline-flex rounded-lg border border-stone-200 bg-stone-50 p-0.5">
              {(['template', 'agent'] as OutputKind[]).map((kind) => (
                <button
                  key={kind}
                  type="button"
                  onClick={() => setOutputKind(kind)}
                  className={`px-4 py-1.5 text-xs font-medium rounded-md transition-colors ${
                    outputKind === kind
                      ? 'bg-white text-stone-800 shadow-sm'
                      : 'text-stone-500 hover:text-stone-700'
                  }`}
                >
                  {kind === 'template' ? 'AgentTemplate' : 'Agent'}
                </button>
              ))}
            </div>
            <p className="mt-1.5 text-xs text-stone-400">
              {outputKind === 'template'
                ? 'Generates a reusable AgentTemplate; create Agents referencing it later.'
                : 'Generates a runnable Agent directly.'}
            </p>
          </div>

          {/* Identity */}
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label htmlFor="name" className={labelClass}>Name</label>
              <input
                id="name"
                type="text"
                value={resourceName}
                onChange={(e) => {
                  const sanitized = e.target.value
                    .toLowerCase()
                    .replace(/\s+/g, '-')
                    .replace(/[^a-z0-9\-.]/g, '');
                  setResourceName(sanitized);
                }}
                required
                placeholder={outputKind === 'template' ? 'my-template' : 'my-agent'}
                className={inputClass}
              />
            </div>
            <div>
              <label className={labelClass}>Namespace</label>
              <div className="px-3 py-2 rounded-lg border border-stone-200 bg-stone-50 text-sm text-stone-600 font-mono">
                {namespace}
              </div>
            </div>
          </div>

          {/* Workspace + SA */}
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label htmlFor="workspaceDir" className={labelClass}>Workspace Directory</label>
              <input
                id="workspaceDir"
                type="text"
                value={workspaceDir}
                onChange={(e) => setWorkspaceDir(e.target.value)}
                required
                className={monoInputClass}
              />
            </div>
            <div>
              <label htmlFor="serviceAccountName" className={labelClass}>Service Account</label>
              <input
                id="serviceAccountName"
                type="text"
                value={serviceAccountName}
                onChange={(e) => setServiceAccountName(e.target.value)}
                required
                className={monoInputClass}
              />
            </div>
          </div>

          {/* Executor Image */}
          <div>
            <span className={labelClass}>
              Executor Image <span className="normal-case tracking-normal text-stone-300">(worker container, required)</span>
            </span>
            {executorImageCandidates.length === 0 ? (
              <p className="text-xs text-stone-400">No images available for this slot.</p>
            ) : (
              <div className="grid grid-cols-1 md:grid-cols-2 gap-2">
                {executorImageCandidates.map((img) =>
                  renderImageCard(img, executorImage === img.image, () =>
                    setExecutorImage(executorImage === img.image ? '' : img.image),
                  ),
                )}
              </div>
            )}
          </div>

          {/* Agent Image */}
          <div>
            <span className={labelClass}>
              Agent Image <span className="normal-case tracking-normal text-stone-300">(init container, optional)</span>
            </span>
            {agentImageCandidates.length === 0 ? (
              <p className="text-xs text-stone-400">No images available for this slot.</p>
            ) : (
              <div className="grid grid-cols-1 md:grid-cols-2 gap-2">
                {agentImageCandidates.map((img) =>
                  renderImageCard(img, agentImage === img.image, () =>
                    setAgentImage(agentImage === img.image ? '' : img.image),
                  ),
                )}
              </div>
            )}
          </div>

          {/* Skills */}
          <div>
            <span className={labelClass}>Skills <span className="normal-case tracking-normal text-stone-300">(multi-select)</span></span>
            {skills.length === 0 ? (
              <p className="text-xs text-stone-400">No skills in this registry.</p>
            ) : (
              <div className="space-y-2">
                {skills.map((s) =>
                  renderAssetRow(
                    s.name,
                    selectedSkills.has(s.name),
                    () => toggle(setSelectedSkills, s.name),
                    s.name,
                    s.repository,
                    s.phase,
                  ),
                )}
              </div>
            )}
          </div>

          {/* Plugins */}
          <div>
            <span className={labelClass}>Plugins <span className="normal-case tracking-normal text-stone-300">(multi-select)</span></span>
            {plugins.length === 0 ? (
              <p className="text-xs text-stone-400">No plugins in this registry.</p>
            ) : (
              <div className="space-y-2">
                {plugins.map((p) =>
                  renderAssetRow(
                    p.name,
                    selectedPlugins.has(p.name),
                    () => toggle(setSelectedPlugins, p.name),
                    p.name,
                    p.package,
                    p.phase,
                  ),
                )}
              </div>
            )}
          </div>

          {createMutation.isError && (
            <div className="bg-red-50 border border-red-200 rounded-lg p-4">
              <p className="text-red-700 text-sm">{(createMutation.error as Error).message}</p>
            </div>
          )}

          <div className="flex justify-end gap-3 pt-2">
            <Link
              to={`/registries/${namespace}/${name}`}
              className="px-4 py-2.5 text-sm font-medium text-stone-600 bg-white shadow-ring rounded-lg hover:shadow-card transition-all"
            >
              Cancel
            </Link>
            <button
              type="submit"
              disabled={createMutation.isPending || !isValid}
              className="px-5 py-2.5 text-sm font-medium text-white bg-primary-600 rounded-lg hover:bg-primary-700 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
            >
              {createMutation.isPending
                ? 'Creating…'
                : outputKind === 'template'
                ? 'Create AgentTemplate'
                : 'Create Agent'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

export default RegistryAssemblePage;
