"""Read the scripts selected by a thin GitHub workflow adapter for contract assertions."""
from pathlib import Path
import re

import yaml


def expanded_run(root, command):
    """Include selected workflow commands and owner scripts, without executing them."""
    root = Path(root).resolve()
    content = [command]
    selected = re.findall(r'workflows/shared/launch\.py\s+(workflows/[A-Za-z0-9_./-]+\.yml)', command)
    for name in selected:
        workflow = (root / name).resolve(strict=True)
        if not workflow.is_relative_to(root / 'workflows'):
            raise ValueError('Workflow path escapes its owner tree')
        document = yaml.safe_load(workflow.read_text())
        for step in document.get('steps', []):
            operation = step.get('cmd', '')
            content.append(operation)
            scripts = re.findall(r'workflows/[A-Za-z0-9_./-]+\.(?:sh|ps1|py)', operation)
            for script_name in scripts:
                script = (root / script_name).resolve(strict=True)
                if not script.is_relative_to(root / 'workflows'):
                    raise ValueError('Script path escapes its owner tree')
                content.append(script.read_text())
    return '\n'.join(content)
