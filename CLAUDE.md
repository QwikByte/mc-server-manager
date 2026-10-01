## Project Overview
This project is an all-in-one solution for creating and managing minecraft servers of all types or even whole networks.
The idea is that we have a frontend / master agent that can be connected to other agents that are installed on different dedicated servers via a secured connection.
Agents must only accept commands/tasks from master (means from the administration panel) or directly local with an own cli.

## Architecture
Always work by package by feature principle. Dont mix. If you find something kinda unorganized, then clean it up.
There should be a master agent that directly communicates with the frontend (adminpanel) and collects / manages the other agents. (So we kinda need 2 different installable programs)

## Coding Rules
- Think before you write code. If you add something new, always make a plan before acting.
- If you need more information or something is unclear. Ask. Dont guess.
- Always clean your code as much as possible.
- If your written code can be like 40 lines instead of your current 100. Rewrite it.

## Design System
- For fontend always use a modern ui theme with best practises (use frontend plugin)

## Utility
- Use Context7 MCP if needed
- Always use up-to-date code
- Security must be focused on, so think about it before implement something
