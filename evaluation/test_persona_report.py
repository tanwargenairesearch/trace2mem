import hashlib
import json
from pathlib import Path
import tempfile
import unittest

from persona_report import copy_corpus, token_delta


class ReportEvidenceTests(unittest.TestCase):
    def fixture(self, root, path='notes/fact.md'):
        source=root/'source';folder=source/'persona';folder.mkdir(parents=True)
        file=folder/'notes/fact.md';file.parent.mkdir();file.write_bytes(b'fact')
        entry={'path':path,'size':4,'sha256':hashlib.sha256(b'fact').hexdigest()}
        manifest=json.dumps({'files':[entry]}).encode()
        (folder/'manifest.json').write_bytes(manifest)
        frozen={'manifest_sha256':{'persona':hashlib.sha256(manifest).hexdigest()}}
        return source,frozen

    def test_copies_only_verified_manifest_files(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp);source,frozen=self.fixture(root)
            (source/'persona/.env').write_text('private sidecar')
            copy_corpus(source,root/'out',frozen)
            self.assertEqual((root/'out/persona/notes/fact.md').read_bytes(),b'fact')
            self.assertFalse((root/'out/persona/.env').exists())

    def test_rejects_changed_manifest_or_content(self):
        for name in ('manifest.json','notes/fact.md'):
            with self.subTest(name=name),tempfile.TemporaryDirectory() as tmp:
                root=Path(tmp);source,frozen=self.fixture(root)
                with (source/'persona'/name).open('ab') as f:f.write(b' ')
                with self.assertRaises(ValueError):copy_corpus(source,root/'out',frozen)

    def test_rejects_unsafe_paths(self):
        for path in ('../secret','/secret','notes/../secret','notes//fact.md','manifest.json'):
            with self.subTest(path=path),tempfile.TemporaryDirectory() as tmp:
                root=Path(tmp);source,frozen=self.fixture(root,path)
                with self.assertRaises(ValueError):copy_corpus(source,root/'out',frozen)

    def test_no_precise_delta_for_uncertain_usage(self):
        measured={'accounted_tokens':100,'missing_usage_trials':0,'unresolved_usage_trials':0}
        self.assertEqual(token_delta(measured,measured),'+0.0%')
        for key in ('missing_usage_trials','unresolved_usage_trials'):
            uncertain={**measured,key:1}
            self.assertEqual(token_delta(uncertain,measured),'N/A')
            self.assertEqual(token_delta(measured,uncertain),'N/A')


if __name__=='__main__':unittest.main()
