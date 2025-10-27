"""Sphinx configuration for the Identity Service documentation."""

from __future__ import annotations

import os
import sys
from datetime import datetime

ROOT_DIR = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
sys.path.insert(0, ROOT_DIR)


project = "Identity Service"
author = "Platform Team"
copyright = f"{datetime.now():%Y}, {author}"
release = "0.1.0"

extensions = [
    "sphinx.ext.autodoc",
    "sphinx.ext.napoleon",
    "sphinx.ext.viewcode",
    "sphinx_autodoc_typehints",
]

autodoc_typehints = "description"
autodoc_preserve_defaults = True
napoleon_google_docstring = True
napoleon_numpy_docstring = False

templates_path = ["_templates"]
exclude_patterns: list[str] = ["_build", "Thumbs.db", ".DS_Store"]

html_theme = "alabaster"
html_static_path = ["_static"]
