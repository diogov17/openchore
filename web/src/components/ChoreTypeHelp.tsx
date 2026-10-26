import { useId, useState } from 'react';
import { Info } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import styles from './ChoreTypeHelp.module.css';

/** Shared disclosure for the chore category fields in creation and editing. */
export function ChoreTypeHelp({ title, legend = false, labelFor }: { title: string; legend?: boolean; labelFor?: string }) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const id = useId();
  const Heading = legend ? 'legend' : 'div';
  const types = [
    ['design.category.essential', 'required'],
    ['design.category.daily', 'core'],
    ['design.category.bonus', 'bonus'],
    ['chores.independent', 'independent'],
    ['design.category.home', 'home'],
  ];
  return <>
    <Heading className={styles.heading}>
      {labelFor ? <label htmlFor={labelFor}>{title}</label> : <span>{title}</span>}
      <button type="button" className={styles.info} aria-label={t('chores.typeHelp.title')} aria-expanded={open} aria-controls={id} onClick={() => setOpen(!open)}>
        <Info size={16} aria-hidden="true" />
      </button>
    </Heading>
    {open && <div id={id} className={styles.panel} role="region" aria-label={t('chores.typeHelp.title')}>
      <dl>{types.map(([label, key]) => <div key={key}><dt>{t(label)}</dt><dd>{t(`chores.typeHelp.${key}`)}</dd></div>)}</dl>
      <p>{t('chores.typeHelp.schedule')}</p>
    </div>}
  </>;
}
