
Prompt: Extract all detail from the attached Anthropic Compliance API PDF document and output it as well-formed Markdown. Double check your work. This will be used to feed Gemini so make sure all tables, diagrams, etc are well formed as Markdown or at least in a format that can be understood by an LLM.

Suggested python script:

# To run this code you need to install the following dependencies:
# pip install google-genai

import os
from google import genai
from google.genai import types


def generate():
    client = genai.Client(
        api_key=os.environ.get("GEMINI_API_KEY"),
    )

    model = "gemini-3.1-pro-preview"
    contents = [
        types.Content(
            role="user",
            parts=[
                types.Part.from_bytes(
                    mime_type="application/pdf",
                    data=base64.b64decode(
                        """<Drive file: 1spl7ygAWAwUFnWuCSZgCkRZ7gV5399Nt>"""
                    ),
                ),
                types.Part.from_text(text="""
Extract all detail from the attached Anthropic Compliance API PDF document and output it as well-formed Markdown. Double check your work. This will be used to feed Gemini so make sure all tables, diagrams, etc are well formed as Markdown or at least in a format that can be understood by an LLM."""),
            ],
        ),
        types.Content(
            role="model",
            parts=[
                types.Part.from_text(text="""BLAH"""),
            ],
        ),
        types.Content(
            role="user",
            parts=[
                types.Part.from_text(text="""INSERT_INPUT_HERE"""),
            ],
        ),
    ]
    tools = [
        types.Tool(googleSearch=types.GoogleSearch(
        )),
    ]
    generate_content_config = types.GenerateContentConfig(
        temperature=0.65,
        thinking_config=types.ThinkingConfig(
            thinking_level="HIGH",
        ),
        tools=tools,
    )

    for chunk in client.models.generate_content_stream(
        model=model,
        contents=contents,
        config=generate_content_config,
    ):
        if text := chunk.text:
            print(text, end="")

if __name__ == "__main__":
    generate()



