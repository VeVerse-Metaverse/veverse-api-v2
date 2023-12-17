I give you a JSON describing a virtual world composed of entities: Locations, NPCs, Players and Objects.
You give me a play act script in return so we can play it in the video game.
NPCs, Players and Objects are in locations.
NPCs can speak to Players and each other.
Play act is a FSM states making NPCs perform actions one by one in order.
State actions are simple such as "say" used to say a simple phrase or even a short story from their life, "use", etc.
Most actions have a target, e.g. say action target is a subject to say to.
The say action metadata should be in SSML format.
Action object is the object used to perform the action on target.
Metadata is additional data on the action such as phrase to say or other important data.
Each action must have an NPC who performs it.
NPCs can't use objects they don't have, but they can do a spawn action to pop in new objects.
I provide an input with description of the scene and entities.
Output must be list of actions, performed only by NPCs.
Give me output with the play script.
Minimise other prose except the script JSON.
Empty fields (including empty strings) must be omitted.
Each state output fields supported are:
- a - action type,
- c - NPC performing the action,
- t - target of the action,
- o - item used in the action,
- m - additional data on the action such as a phrase to say,
- e - a text smile.
NPCs spawn in locations as defined in input.
NPCs should mostly speak to each other with funny silly jokes and react to these jokes.
NPCs can emote within a say action or as a separate emote action.
---
Input:
```json
{
  "context": "A modern sitcom taking place in the rented office of a small company",
  "locations": [
    {
      "name": "Office",
      "description": "An office room with a desk, a chair, a computer and other office furniture",
      "links": [
        {
          "location": "Kitchen",
          "description": "A door to the kitchen"
        }
      ],
      "entities": {
        "npcs": [
          {
            "name": "Alice",
            "description": "A nice-looking secretary, flirting and naughty, doesn't understand philosophy too much but loves Bob anyway",
            "location": "Office"
          },
          {
            "name": "Bob",
            "description": "A nerd, afraid of women, shy, likes to eat, philosopher, says clever phrases",
            "location": "Kitchen"
          }
        ],
        "players": [
          {
            "name": "Hackerman",
            "location": "Office"
          }
        ],
        "objects": [
          {
            "name": "Microwave",
            "location": "Kitchen"
          },
          {
            "name": "Fridge",
            "location": "Kitchen"
          }
        ]
      }
    },
    {
      "name": "Kitchen",
      "description": "An office kitchen with a microwave, a fridge, a sink and other kitchen furniture",
      "links": [
        {
          "location": "Office",
          "description": "A door to the office"
        }
      ],
      "entities": {
        "objects": [
          {
            "name": "Microwave",
            "location": "Kitchen"
          },
          {
            "name": "Fridge",
            "location": "Kitchen"
          }
        ]
      }
    }
  ]
}

```
---
Output (maximum of 10 actions):
