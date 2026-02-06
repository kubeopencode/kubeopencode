import { useState, useEffect } from 'react';
import { formatRelativeTime, formatFullTime } from '../utils/time';

interface TimeAgoProps {
  date: string | Date;
  className?: string;
}

function TimeAgo({ date, className }: TimeAgoProps) {
  const [, setTick] = useState(0);

  // Re-render periodically to keep the relative time fresh
  useEffect(() => {
    const interval = setInterval(() => setTick((t) => t + 1), 30000);
    return () => clearInterval(interval);
  }, []);

  return (
    <span className={className} title={formatFullTime(date)}>
      {formatRelativeTime(date)}
    </span>
  );
}

export default TimeAgo;
