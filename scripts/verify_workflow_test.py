import re
import unittest
from pathlib import Path


class VerifyWorkflowTest(unittest.TestCase):
    def test_race_gate_covers_all_go_packages(self):
        workflow = Path(__file__).resolve().parents[1] / ".github/workflows/verify.yml"
        # 采样器在 handler、限流器在 pkg；只检查 repository/service/server 会漏掉它们。
        commands = re.findall(r"^\s+go test -race (.+)$", workflow.read_text(), re.MULTILINE)
        self.assertTrue(commands, "Verify 工作流必须执行 race 检查")
        self.assertTrue(
            any("./..." in command.split() for command in commands),
            "race 门禁必须覆盖全部 Go 包，包括 handler 与 pkg/rate",
        )


if __name__ == "__main__":
    unittest.main()
