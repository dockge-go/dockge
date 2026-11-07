"""冒烟脚本自身的安全边界测试。"""
import io
import json
import os
import unittest
from unittest.mock import patch

from smoke import Smoke


class Response(io.BytesIO):
    code = 200


class SmokeTests(unittest.TestCase):
    def test_credentials_have_no_default(self):
        with patch.dict(os.environ, {}, clear=True), patch.object(Smoke, 'request'), patch.object(Smoke, 'check'):
            with self.assertRaisesRegex(ValueError, '没有默认密码'):
                Smoke().run()

    def test_json_escapes_credentials(self):
        password = 'p"ass\\word123'
        def respond(req, **kwargs):
            self.assertEqual(json.loads(req.data)['password'], password)
            return Response(b'{"data":{}}')
        with patch.dict(os.environ, {}, clear=True), patch('smoke.urlopen', side_effect=respond):
            Smoke().request('POST', '/login', {'password': password}, auth=False)

    def test_cleanup_only_owned_stack_and_preserves_failed_state(self):
        s = Smoke()
        with patch.object(s, 'request') as request:
            s.cleanup()
            request.assert_not_called()
            s.created = True
            request.side_effect = RuntimeError('runtime failed')
            with self.assertRaises(RuntimeError):
                s.cleanup()
            self.assertTrue(s.created)
            request.side_effect = None
            s.cleanup()
            self.assertFalse(s.created)
            request.assert_called_with('DELETE', '/stacks/' + s.stack)

    def test_non_json_success_is_not_silently_accepted(self):
        with patch('smoke.urlopen', return_value=Response(b'not json')):
            with self.assertRaisesRegex(RuntimeError, 'JSON'):
                Smoke().request('GET', '/me')

    def test_operation_stream_is_not_json(self):
        output = b' Container smoke-web-1 Started\n'
        with patch('smoke.urlopen', return_value=Response(output)) as open_request:
            self.assertEqual(Smoke().request('POST', '/stacks/owned/start', stream=True), output.decode())
            self.assertEqual(open_request.call_args.kwargs['timeout'], 660)

    def test_operation_stream_error_preserves_runtime_details(self):
        output = b'pull access denied\n\x1b[31m[error] compose start failed (exit 1)\x1b[0m\n'
        with patch('smoke.urlopen', return_value=Response(output)):
            with self.assertRaisesRegex(RuntimeError, 'pull access denied'):
                Smoke().request('POST', '/stacks/owned/start', stream=True)

    def test_unique_stack_names(self):
        self.assertNotEqual(Smoke().stack, Smoke().stack)


if __name__ == '__main__':
    unittest.main()
