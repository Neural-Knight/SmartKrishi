"""
Simple Graph stub for planning and execution.
"""
class Graph:
	def __init__(self):
		self._nodes = {}
		self._edges = []
		self._entry = None
	def add_node(self, name, func):
		self._nodes[name] = func
	def add_edge(self, src, dst):
		self._edges.append((src, dst))
	def set_entry_point(self, name):
		self._entry = name
	def compile(self):
		return self
	def invoke(self, state):
		# traverse from entry through edges until None
		current = self._entry
		while current is not None:
			fn = self._nodes.get(current)
			if fn:
				state = fn(state)
			# find next node
			next_node = None
			for src, dst in self._edges:
				if src == current:
					next_node = dst
					break
			current = next_node
		return state
END = None
from .state  import State
from .nodes  import planner, main_agent, checker

g = Graph()
g.add_node("planner",       planner.planner_node)
g.add_node("main_agent",    main_agent.main_agent_node)
g.add_node("error_checker", checker.checker_node)
g.add_edge("planner",       "main_agent")
g.add_edge("main_agent",    "error_checker")
g.add_edge("error_checker", END)
g.set_entry_point("planner")

APP = g.compile()
