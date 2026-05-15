import React, { useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import api from '../api/client';
import SearchableSelect from './SearchableSelect';

interface Props {
  outputKind: 'agent' | 'template';
}

function RegistryAssemblerBanner({ outputKind }: Props) {
  const navigate = useNavigate();
  const [selected, setSelected] = useState('');

  const { data } = useQuery({
    queryKey: ['all-registries-for-assembler'],
    queryFn: () => api.listAllRegistries({ limit: 200, sortOrder: 'asc' }),
  });

  const options = useMemo(() => {
    const list = data?.registries || [];
    return [
      { value: '', label: 'Select a registry…' },
      ...list.map((r) => ({
        value: `${r.namespace}/${r.name}`,
        label: `${r.namespace}/${r.name}`,
      })),
    ];
  }, [data]);

  const handleGo = () => {
    if (!selected) return;
    const [ns, name] = selected.split('/');
    navigate(`/registries/${ns}/${name}/assemble?output=${outputKind}`);
  };

  return (
    <div className="bg-primary-50/40 border border-primary-100 rounded-lg px-4 py-3 flex items-center gap-3">
      <div className="flex-1 min-w-0">
        <p className="text-xs font-semibold text-stone-700">
          Assemble from a Registry
        </p>
        <p className="text-[11px] text-stone-500 mt-0.5">
          Pick a Registry to choose images, skills, and plugins from a catalog.
        </p>
      </div>
      <div className="w-64">
        <SearchableSelect
          id="assembler-registry"
          value={selected}
          onChange={setSelected}
          options={options}
          placeholder="Select a registry…"
        />
      </div>
      <button
        type="button"
        onClick={handleGo}
        disabled={!selected}
        className="px-3 py-1.5 text-xs font-medium text-white bg-primary-600 rounded-lg hover:bg-primary-700 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
      >
        Open Assembler
      </button>
    </div>
  );
}

export default RegistryAssemblerBanner;
